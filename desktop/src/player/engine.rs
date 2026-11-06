//! libmpv in-process: decoding, A/V sync, CENC decryption and seeking stay in
//! mpv; a render thread draws each new frame as BGRA, the pixel format GPUI's
//! image atlas uses, so frames go to the GPU without a CPU swizzle.

use std::sync::atomic::{AtomicBool, AtomicU32, AtomicU64, Ordering};
use std::sync::{Arc, Condvar, Mutex};
use std::thread::JoinHandle;

use anyhow::{Context as _, Result};
use gpui_kit::RenderImage;
use image::{Frame, RgbaImage};
use rsmpv::render::{RenderContext, SwPixelFormat};
use rsmpv::{EndFileReason, Event, Format, Mpv, PropertyData};

/// What the UI needs to know from mpv.
#[derive(Debug, Clone)]
pub enum MpvEvent {
    TimePos(f64),
    Duration(f64),
    Paused(bool),
    Buffered(f64),
    Volume(f64),
    Muted(bool),
    Seeking(bool),
    Loaded,
    Ended { eof: bool, error: Option<String> },
}

/// One file to play.
#[derive(Debug, Clone, Default)]
pub struct LoadRequest {
    pub url: String,
    pub audio_url: String,
    /// Hex AES-128 key for a CENC-protected stream.
    pub key: String,
    pub start: f64,
}

/// What the debug panel shows. mpv's properties are `None` while nothing is loaded.
#[derive(Debug, Clone, Default)]
pub struct Stats {
    pub hwdec: Option<String>,
    pub video_codec: Option<String>,
    pub audio_codec: Option<String>,
    pub source: (u32, u32),
    pub output: (u32, u32),
    pub container_fps: Option<f64>,
    pub estimated_fps: Option<f64>,
    pub dropped: Option<i64>,
    pub decoder_dropped: Option<i64>,
    pub cache_seconds: Option<f64>,
    pub video_bitrate: Option<f64>,
    /// Frames the render thread has drawn.
    pub rendered: u64,
}

/// Shared between the UI, mpv's update callback and the render thread.
struct Shared {
    dirty: Mutex<bool>,
    wake_render: Condvar,
    quit: AtomicBool,
    viewport: (AtomicU32, AtomicU32),
    video: (AtomicU32, AtomicU32),
    frame: Mutex<Option<Arc<RenderImage>>>,
    output: (AtomicU32, AtomicU32),
    rendered: AtomicU64,
    notify_pending: AtomicBool,
    notify: Box<dyn Fn() + Send + Sync>,
}

impl Shared {
    fn kick_render(&self) {
        *self.dirty.lock().unwrap() = true;
        self.wake_render.notify_one();
    }

    /// Coalesces bursts of frames and events into one UI wakeup.
    fn wake_ui(&self) {
        if !self.notify_pending.swap(true, Ordering::AcqRel) {
            (self.notify)();
        }
    }
}

pub struct Engine {
    mpv: Arc<Mpv>,
    shared: Arc<Shared>,
    render: Option<JoinHandle<()>>,
    pending_audio: Mutex<String>,
}

// keep-open holds the last frame at the end, so mpv reports the end through
// `eof-reached` instead of an END_FILE event.
const OBSERVED: [(&str, Format); 8] = [
    ("time-pos", Format::Double),
    ("duration", Format::Double),
    ("pause", Format::Flag),
    ("demuxer-cache-time", Format::Double),
    ("volume", Format::Double),
    ("mute", Format::Flag),
    ("seeking", Format::Flag),
    ("eof-reached", Format::Flag),
];

impl Engine {
    /// `notify` is called from mpv's threads whenever a frame or events are
    /// waiting; the UI then calls [`Engine::take_frame`] and [`Engine::drain_events`].
    pub fn new(volume: f64, notify: impl Fn() + Send + Sync + 'static) -> Result<Engine> {
        let mpv = Mpv::builder()?
            .set_property("vo", "libmpv")?
            .set_property("hwdec", "auto-copy")?
            // The software renderer's YUV->RGB conversion: zimg's thread pool
            // costs ~4x one thread here (77% vs 19% CPU at 1080p30).
            .set_property("zimg-threads", 1i64)?
            .set_property("keep-open", "yes")?
            .set_property("idle", "yes")?
            .set_property("input-default-bindings", false)?
            .set_property("osc", false)?
            .set_property("ytdl", false)?
            .set_property("cache", "yes")?
            .set_property("demuxer-max-bytes", "150MiB")?
            .set_property("demuxer-readahead-secs", 30.0)?
            .set_property("volume", volume)?
            .build()
            .context("starting libmpv")?;
        for (i, (name, format)) in OBSERVED.iter().enumerate() {
            mpv.observe_property(i as u64 + 1, name, *format)?;
        }
        let mpv = Arc::new(mpv);
        let shared = Arc::new(Shared {
            dirty: Mutex::new(false),
            wake_render: Condvar::new(),
            quit: AtomicBool::new(false),
            viewport: (AtomicU32::new(0), AtomicU32::new(0)),
            video: (AtomicU32::new(0), AtomicU32::new(0)),
            frame: Mutex::new(None),
            output: (AtomicU32::new(0), AtomicU32::new(0)),
            rendered: AtomicU64::new(0),
            notify_pending: AtomicBool::new(false),
            notify: Box::new(notify),
        });
        {
            let shared = shared.clone();
            mpv.set_wakeup_callback(move || shared.wake_ui());
        }
        let mut ctx = RenderContext::new_software(mpv.clone())
            .context("creating the mpv software renderer")?;
        {
            let shared = shared.clone();
            ctx.set_update_callback(move || shared.kick_render());
        }
        let render = {
            let shared = shared.clone();
            std::thread::Builder::new()
                .name("ohg-video".into())
                .spawn(move || render_loop(ctx, &shared))?
        };
        Ok(Engine {
            mpv,
            shared,
            render: Some(render),
            pending_audio: Mutex::new(String::new()),
        })
    }

    pub fn load(&self, req: &LoadRequest) -> Result<()> {
        let lavf = if req.key.is_empty() {
            String::new()
        } else {
            format!("decryption_key={}", req.key)
        };
        self.mpv.set_property("demuxer-lavf-o", lavf.as_str())?;
        let start = if req.start > 0.0 {
            format!("{:.3}", req.start)
        } else {
            "none".into()
        };
        self.mpv.set_property("start", start.as_str())?;
        *self.pending_audio.lock().unwrap() = req.audio_url.clone();
        self.mpv
            .command(&["loadfile", req.url.as_str(), "replace"])?;
        Ok(())
    }

    pub fn stop(&self) {
        let _ = self.mpv.command(&["stop"]);
        *self.shared.frame.lock().unwrap() = None;
    }

    pub fn set_paused(&self, paused: bool) {
        let _ = self.mpv.set_property("pause", paused);
    }

    pub fn seek_to(&self, seconds: f64) {
        let _ = self
            .mpv
            .command(&["seek", &format!("{seconds:.3}"), "absolute"]);
    }

    pub fn seek_by(&self, seconds: f64) {
        let _ = self
            .mpv
            .command(&["seek", &format!("{seconds:.3}"), "relative"]);
    }

    pub fn set_volume(&self, volume: f64) {
        let _ = self.mpv.set_property("volume", volume.clamp(0.0, 130.0));
    }

    pub fn set_muted(&self, muted: bool) {
        let _ = self.mpv.set_property("mute", muted);
    }

    pub fn dropped_frames(&self) -> i64 {
        self.mpv
            .get_property::<i64>("frame-drop-count")
            .unwrap_or(0)
            + self
                .mpv
                .get_property::<i64>("decoder-frame-drop-count")
                .unwrap_or(0)
    }

    pub fn hwdec_current(&self) -> String {
        self.mpv
            .get_property::<String>("hwdec-current")
            .unwrap_or_default()
    }

    pub fn stats(&self) -> Stats {
        let sh = &self.shared;
        Stats {
            hwdec: self.mpv.get_property::<String>("hwdec-current").ok(),
            video_codec: self.mpv.get_property::<String>("video-codec").ok(),
            audio_codec: self.mpv.get_property::<String>("audio-codec-name").ok(),
            source: (
                sh.video.0.load(Ordering::Relaxed),
                sh.video.1.load(Ordering::Relaxed),
            ),
            output: (
                sh.output.0.load(Ordering::Relaxed),
                sh.output.1.load(Ordering::Relaxed),
            ),
            container_fps: self.mpv.get_property::<f64>("container-fps").ok(),
            estimated_fps: self.mpv.get_property::<f64>("estimated-vf-fps").ok(),
            dropped: self.mpv.get_property::<i64>("frame-drop-count").ok(),
            decoder_dropped: self
                .mpv
                .get_property::<i64>("decoder-frame-drop-count")
                .ok(),
            cache_seconds: self.mpv.get_property::<f64>("demuxer-cache-duration").ok(),
            video_bitrate: self.mpv.get_property::<f64>("video-bitrate").ok(),
            rendered: sh.rendered.load(Ordering::Relaxed),
        }
    }

    /// The video's display size, `(0, 0)` until mpv knows it.
    pub fn video_size(&self) -> (u32, u32) {
        (
            self.shared.video.0.load(Ordering::Relaxed),
            self.shared.video.1.load(Ordering::Relaxed),
        )
    }

    /// The device-pixel size the video is shown at; frames are rendered at
    /// this size (never above the source) so mpv does the one scale.
    pub fn set_viewport(&self, width: u32, height: u32) {
        let (w, h) = &self.shared.viewport;
        if w.swap(width, Ordering::Relaxed) != width || h.swap(height, Ordering::Relaxed) != height
        {
            self.shared.kick_render();
        }
    }

    pub fn take_frame(&self) -> Option<Arc<RenderImage>> {
        self.shared.frame.lock().unwrap().take()
    }

    pub fn drain_events(&self) -> Vec<MpvEvent> {
        self.shared.notify_pending.store(false, Ordering::Release);
        let mut out = Vec::new();
        while let Some(event) = self.mpv.poll_event() {
            match event {
                Event::PropertyChange { name, data, .. } => {
                    if let Some(e) = property_event(&name, data) {
                        out.push(e);
                    }
                }
                Event::FileLoaded => {
                    let audio = std::mem::take(&mut *self.pending_audio.lock().unwrap());
                    if !audio.is_empty() {
                        let _ = self.mpv.command(&["audio-add", audio.as_str(), "select"]);
                    }
                    out.push(MpvEvent::Loaded);
                }
                Event::VideoReconfig => {
                    let w = self.mpv.get_property::<i64>("dwidth").unwrap_or(0).max(0) as u32;
                    let h = self.mpv.get_property::<i64>("dheight").unwrap_or(0).max(0) as u32;
                    self.shared.video.0.store(w, Ordering::Relaxed);
                    self.shared.video.1.store(h, Ordering::Relaxed);
                    self.shared.kick_render();
                }
                Event::EndFile { reason, error, .. } => match reason {
                    EndFileReason::Eof => out.push(MpvEvent::Ended {
                        eof: true,
                        error: None,
                    }),
                    EndFileReason::Error => out.push(MpvEvent::Ended {
                        eof: false,
                        error: Some(error.map(|e| e.to_string()).unwrap_or_default()),
                    }),
                    _ => {}
                },
                _ => {}
            }
        }
        out
    }
}

impl Drop for Engine {
    fn drop(&mut self) {
        self.shared.quit.store(true, Ordering::Release);
        self.shared.kick_render();
        if let Some(t) = self.render.take() {
            let _ = t.join();
        }
        self.mpv.clear_wakeup_callback();
    }
}

fn property_event(name: &str, data: PropertyData) -> Option<MpvEvent> {
    let f = match data {
        PropertyData::Double(v) => Some(v),
        _ => None,
    };
    let b = match data {
        PropertyData::Flag(v) => Some(v),
        _ => None,
    };
    Some(match name {
        "time-pos" => MpvEvent::TimePos(f?),
        "duration" => MpvEvent::Duration(f?),
        "demuxer-cache-time" => MpvEvent::Buffered(f?),
        "volume" => MpvEvent::Volume(f?),
        "pause" => MpvEvent::Paused(b?),
        "eof-reached" if b? => MpvEvent::Ended {
            eof: true,
            error: None,
        },
        "eof-reached" => return None,
        "mute" => MpvEvent::Muted(b?),
        "seeking" => MpvEvent::Seeking(b?),
        _ => return None,
    })
}

/// Fits the video's display size into the viewport, never upscaling past the source.
pub fn fit(video: (u32, u32), viewport: (u32, u32)) -> (u32, u32) {
    let (vw, vh) = video;
    let (pw, ph) = viewport;
    if vw == 0 || vh == 0 {
        return (0, 0);
    }
    if pw == 0 || ph == 0 {
        return (vw, vh);
    }
    let scale = (pw as f64 / vw as f64).min(ph as f64 / vh as f64).min(1.0);
    let w = ((vw as f64 * scale).round() as u32).max(2) & !1;
    let h = ((vh as f64 * scale).round() as u32).max(2) & !1;
    (w, h)
}

fn render_loop(mut ctx: RenderContext<Arc<Mpv>>, shared: &Shared) {
    let mut last = (0, 0);
    loop {
        {
            let mut dirty = shared.dirty.lock().unwrap();
            while !*dirty && !shared.quit.load(Ordering::Acquire) {
                dirty = shared.wake_render.wait(dirty).unwrap();
            }
            *dirty = false;
        }
        if shared.quit.load(Ordering::Acquire) {
            return;
        }
        let video = (
            shared.video.0.load(Ordering::Relaxed),
            shared.video.1.load(Ordering::Relaxed),
        );
        let viewport = (
            shared.viewport.0.load(Ordering::Relaxed),
            shared.viewport.1.load(Ordering::Relaxed),
        );
        let size = fit(video, viewport);
        let new_frame = ctx.update();
        if !new_frame && size == last {
            continue;
        }
        let (w, h) = size;
        if w == 0 || h == 0 {
            continue;
        }
        let mut buf = vec![0u8; (w * h * 4) as usize];
        if ctx
            .render_software(
                w as i32,
                h as i32,
                SwPixelFormat::Bgr0,
                (w * 4) as usize,
                &mut buf,
            )
            .is_err()
        {
            continue;
        }
        last = size;
        // "bgr0" leaves the fourth byte undefined; GPUI reads it as alpha.
        for px in buf.chunks_exact_mut(4) {
            px[3] = 0xFF;
        }
        let Some(img) = RgbaImage::from_raw(w, h, buf) else {
            continue;
        };
        let image = Arc::new(RenderImage::new(smallvec::smallvec![Frame::new(img)]));
        shared.output.0.store(w, Ordering::Relaxed);
        shared.output.1.store(h, Ordering::Relaxed);
        shared.rendered.fetch_add(1, Ordering::Relaxed);
        *shared.frame.lock().unwrap() = Some(image);
        shared.wake_ui();
    }
}

#[cfg(test)]
mod tests {
    use super::{MpvEvent, PropertyData, fit, property_event};

    #[test]
    fn eof_reached_ends_the_episode() {
        assert!(matches!(
            property_event("eof-reached", PropertyData::Flag(true)),
            Some(MpvEvent::Ended {
                eof: true,
                error: None
            })
        ));
        assert!(property_event("eof-reached", PropertyData::Flag(false)).is_none());
    }

    #[test]
    fn fit_keeps_aspect_and_never_upscales() {
        assert_eq!(fit((720, 1280), (2000, 2000)), (720, 1280));
        assert_eq!(fit((720, 1280), (360, 2000)), (360, 640));
        assert_eq!(fit((1920, 1080), (960, 1080)), (960, 540));
        assert_eq!(fit((0, 0), (100, 100)), (0, 0));
        assert_eq!(fit((720, 1280), (0, 0)), (720, 1280));
    }
}
