//! Cover images: HTTP -> disk cache (`~/.cache/openhongguo/covers`) -> decode
//! -> BGRA `RenderImage`. HEIC, which the service serves for some covers, goes
//! through `ffmpeg` exactly as `internal/tui/cover.go` does it.

use std::collections::{HashMap, VecDeque};
use std::io::Read as _;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::Arc;

use anyhow::{Context as _, Result, bail};
use gpui_kit::*;
use image::{Frame, RgbaImage};

const MAX_BYTES: u64 = 8 << 20;
const MAX_PIXELS: u64 = 40_000_000;
const MAX_LOADED: usize = 400;
const MAX_INFLIGHT: usize = 6;
/// Covers are shown at most ~240 logical px wide; 2x for HiDPI.
const THUMB: (u32, u32) = (480, 720);

enum Slot {
    Loading,
    Ready(Arc<RenderImage>),
    Failed,
}

pub struct Covers {
    http: reqwest::blocking::Client,
    dir: PathBuf,
    ffmpeg: Option<PathBuf>,
    slots: HashMap<String, Slot>,
    lru: VecDeque<String>,
    queue: VecDeque<String>,
    inflight: usize,
    retired: Vec<Arc<RenderImage>>,
}

impl Covers {
    pub fn new() -> Result<Covers> {
        let dir = dirs::cache_dir()
            .context("no cache directory")?
            .join("openhongguo")
            .join("covers");
        std::fs::create_dir_all(&dir)?;
        let http = reqwest::blocking::Client::builder()
            .timeout(std::time::Duration::from_secs(20))
            .build()?;
        Ok(Covers {
            http,
            dir,
            ffmpeg: find_ffmpeg(),
            slots: HashMap::new(),
            lru: VecDeque::new(),
            queue: VecDeque::new(),
            inflight: 0,
            retired: Vec::new(),
        })
    }

    /// The decoded cover, or `None` while it loads (the view is notified
    /// through `cx.observe` when it arrives) or when it failed.
    pub fn get(&mut self, url: &str, cx: &mut Context<Self>) -> Option<Arc<RenderImage>> {
        if url.is_empty() {
            return None;
        }
        match self.slots.get(url) {
            Some(Slot::Ready(img)) => return Some(img.clone()),
            Some(_) => return None,
            None => {}
        }
        self.slots.insert(url.to_string(), Slot::Loading);
        self.queue.push_back(url.to_string());
        self.pump(cx);
        None
    }

    /// Images evicted from the cache; the window frees their atlas tiles.
    pub fn take_retired(&mut self) -> Vec<Arc<RenderImage>> {
        std::mem::take(&mut self.retired)
    }

    fn pump(&mut self, cx: &mut Context<Self>) {
        while self.inflight < MAX_INFLIGHT {
            let Some(url) = self.queue.pop_front() else {
                return;
            };
            self.inflight += 1;
            let job = Job {
                http: self.http.clone(),
                dir: self.dir.clone(),
                ffmpeg: self.ffmpeg.clone(),
                url: url.clone(),
            };
            cx.spawn(async move |this, cx| {
                let result = cx
                    .background_executor()
                    .spawn(async move { job.run() })
                    .await;
                let _ = this.update(cx, |c, cx| c.finish(url, result, cx));
            })
            .detach();
        }
    }

    fn finish(&mut self, url: String, result: Result<RenderImage>, cx: &mut Context<Self>) {
        self.inflight -= 1;
        match result {
            Ok(img) => {
                self.slots.insert(url.clone(), Slot::Ready(Arc::new(img)));
                self.lru.push_back(url);
                while self.lru.len() > MAX_LOADED {
                    let Some(old) = self.lru.pop_front() else {
                        break;
                    };
                    if let Some(Slot::Ready(img)) = self.slots.remove(&old) {
                        self.retired.push(img);
                    }
                }
            }
            Err(err) => {
                log::warn!("cover {url}: {err:#}");
                self.slots.insert(url, Slot::Failed);
            }
        }
        self.pump(cx);
        cx.notify();
    }
}

struct Job {
    http: reqwest::blocking::Client,
    dir: PathBuf,
    ffmpeg: Option<PathBuf>,
    url: String,
}

impl Job {
    fn run(self) -> Result<RenderImage> {
        // Signed cover URLs change their query on every listing; the path names the image.
        let stable = self.url.split('?').next().unwrap_or(&self.url);
        let path = self
            .dir
            .join(sha1_smol::Sha1::from(stable).digest().to_string());
        let data = match std::fs::read(&path) {
            Ok(data) => data,
            Err(_) => {
                let data = self.download()?;
                let data = if is_iso_media(&data) {
                    self.heic_to_png(&data)?
                } else {
                    data
                };
                let tmp = path.with_extension("tmp");
                std::fs::write(&tmp, &data)?;
                std::fs::rename(&tmp, &path)?;
                data
            }
        };
        decode(&data)
    }

    fn download(&self) -> Result<Vec<u8>> {
        let resp = self.http.get(&self.url).send()?;
        if !resp.status().is_success() {
            bail!("HTTP {}", resp.status());
        }
        let mut data = Vec::new();
        resp.take(MAX_BYTES).read_to_end(&mut data)?;
        Ok(data)
    }

    /// The input is a file, not a pipe: ffmpeg seeks back to the image data.
    /// The demuxer is named, drefs are off and only files are allowed.
    fn heic_to_png(&self, data: &[u8]) -> Result<Vec<u8>> {
        let Some(ffmpeg) = &self.ffmpeg else {
            bail!("HEIC cover and no ffmpeg on PATH")
        };
        let tmp = std::env::temp_dir().join(format!(
            "ohg-cover-{}-{}.heic",
            std::process::id(),
            rand_suffix(&self.url)
        ));
        std::fs::write(&tmp, data)?;
        let mut cmd = Command::new(ffmpeg);
        #[cfg(windows)]
        {
            // The app has no console; without this each cover flashes one.
            use std::os::windows::process::CommandExt as _;
            const CREATE_NO_WINDOW: u32 = 0x0800_0000;
            cmd.creation_flags(CREATE_NO_WINDOW);
        }
        let out = cmd
            .args([
                "-hide_banner",
                "-loglevel",
                "error",
                "-protocol_whitelist",
                "file",
                "-f",
                "mov",
                "-enable_drefs",
                "0",
                "-i",
            ])
            .arg(&tmp)
            .args(["-frames:v", "1", "-vf"])
            .arg(format!(
                "scale={}:{}:force_original_aspect_ratio=decrease",
                THUMB.0, THUMB.1
            ))
            .args(["-f", "image2pipe", "-c:v", "png", "pipe:1"])
            .output();
        let _ = std::fs::remove_file(&tmp);
        let out = out?;
        if !out.status.success() || out.stdout.is_empty() {
            bail!("ffmpeg: {}", String::from_utf8_lossy(&out.stderr).trim());
        }
        Ok(out.stdout)
    }
}

fn decode(data: &[u8]) -> Result<RenderImage> {
    let reader = image::ImageReader::new(std::io::Cursor::new(data)).with_guessed_format()?;
    let (w, h) = reader.into_dimensions()?;
    if (w as u64) * (h as u64) > MAX_PIXELS {
        bail!("image is {w}x{h}");
    }
    let img = image::load_from_memory(data)?;
    let img = if img.width() > THUMB.0 || img.height() > THUMB.1 {
        img.thumbnail(THUMB.0, THUMB.1)
    } else {
        img
    };
    let mut rgba = img.into_rgba8();
    // GPUI's atlas is BGRA.
    for px in rgba.chunks_exact_mut(4) {
        px.swap(0, 2);
    }
    let (w, h) = rgba.dimensions();
    let bgra = RgbaImage::from_raw(w, h, rgba.into_raw()).context("image buffer")?;
    Ok(RenderImage::new(smallvec::smallvec![Frame::new(bgra)]))
}

/// An ISO media `ftyp` box: HEIC.
fn is_iso_media(data: &[u8]) -> bool {
    data.len() >= 12 && &data[4..8] == b"ftyp"
}

fn rand_suffix(url: &str) -> String {
    sha1_smol::Sha1::from(url).digest().to_string()[..12].to_string()
}

/// The ffmpeg the macOS and Windows packages ship next to the executable,
/// else one on PATH. An app opened from Finder does not see Homebrew's PATH.
fn find_ffmpeg() -> Option<PathBuf> {
    let name = format!("ffmpeg{}", std::env::consts::EXE_SUFFIX);
    let beside = std::env::current_exe()
        .ok()
        .and_then(|exe| exe.parent().map(Path::to_path_buf));
    let path = std::env::var_os("PATH")
        .map(|p| std::env::split_paths(&p).collect::<Vec<_>>())
        .unwrap_or_default();
    beside
        .into_iter()
        .chain(path)
        .map(|dir| dir.join(&name))
        .find(|p| p.is_file())
}

#[cfg(test)]
mod tests {
    use super::{RgbaImage, decode, is_iso_media};

    #[test]
    fn decodes_png_to_bgra() {
        let mut src = RgbaImage::new(2, 1);
        src.put_pixel(0, 0, image::Rgba([10, 20, 30, 255]));
        let mut png = Vec::new();
        image::DynamicImage::ImageRgba8(src)
            .write_to(&mut std::io::Cursor::new(&mut png), image::ImageFormat::Png)
            .unwrap();
        let img = decode(&png).unwrap();
        assert_eq!(&img.as_bytes(0).unwrap()[..4], &[30, 20, 10, 255]);
        assert!(is_iso_media(b"\0\0\0\x18ftypheic\0\0\0\0"));
        assert!(!is_iso_media(&png));
    }
}
