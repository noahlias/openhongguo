//! The GPUI side of the player: owns the [`Engine`], swaps in each new frame
//! and frees the previous one's atlas tile, and paints the frame letterboxed.

use std::sync::Arc;

use anyhow::Result;
use futures::StreamExt;
use gpui_kit::*;

use super::engine::{Engine, LoadRequest, MpvEvent, Stats};

#[derive(Debug, Clone, Default)]
pub struct PlayState {
    pub position: f64,
    pub duration: f64,
    pub buffered: f64,
    pub paused: bool,
    pub volume: f64,
    pub muted: bool,
    pub seeking: bool,
    pub loaded: bool,
}

#[derive(Debug, Clone)]
pub enum PlayerEvent {
    Loaded,
    Ended { eof: bool, error: Option<String> },
    Paused(bool),
    Seeked,
}

pub struct Player {
    engine: Arc<Engine>,
    frame: Option<Arc<RenderImage>>,
    retired: Vec<Arc<RenderImage>>,
    pub state: PlayState,
    _pump: Task<()>,
}

impl EventEmitter<PlayerEvent> for Player {}

impl Player {
    /// Opens libmpv; the entity is created only once that succeeded.
    pub fn open(volume: f64, cx: &mut App) -> Result<Entity<Player>> {
        let (tx, mut rx) = futures::channel::mpsc::unbounded::<()>();
        let engine = Arc::new(Engine::new(volume, move || {
            let _ = tx.unbounded_send(());
        })?);
        Ok(cx.new(|cx| {
            let pump = cx.spawn(async move |this, cx| {
                while rx.next().await.is_some() {
                    if this.update(cx, |p: &mut Player, cx| p.pump(cx)).is_err() {
                        break;
                    }
                }
            });
            Player {
                engine,
                frame: None,
                retired: Vec::new(),
                state: PlayState {
                    volume,
                    ..Default::default()
                },
                _pump: pump,
            }
        }))
    }

    pub fn load(&mut self, req: &LoadRequest, cx: &mut Context<Self>) -> Result<()> {
        self.state = PlayState {
            volume: self.state.volume,
            muted: self.state.muted,
            ..Default::default()
        };
        self.engine.load(req)?;
        self.engine.set_paused(false);
        cx.notify();
        Ok(())
    }

    pub fn stop(&mut self, cx: &mut Context<Self>) {
        self.engine.stop();
        if let Some(f) = self.frame.take() {
            self.retired.push(f);
        }
        self.state.loaded = false;
        cx.notify();
    }

    pub fn toggle_pause(&mut self) {
        self.engine.set_paused(!self.state.paused);
    }

    pub fn set_paused(&mut self, paused: bool) {
        self.engine.set_paused(paused);
    }

    pub fn seek_by(&mut self, seconds: f64) {
        self.engine.seek_by(seconds);
    }

    pub fn seek_to(&mut self, seconds: f64) {
        self.engine.seek_to(seconds.max(0.0));
    }

    pub fn set_volume(&mut self, volume: f64) {
        self.engine.set_volume(volume);
        if self.state.muted && volume > 0.0 {
            self.engine.set_muted(false);
        }
    }

    pub fn toggle_mute(&mut self) {
        self.engine.set_muted(!self.state.muted);
    }

    pub fn dropped_frames(&self) -> i64 {
        self.engine.dropped_frames()
    }

    pub fn hwdec_current(&self) -> String {
        self.engine.hwdec_current()
    }

    pub fn stats(&self) -> Stats {
        self.engine.stats()
    }

    pub fn video_size(&self) -> (u32, u32) {
        self.engine.video_size()
    }

    pub fn has_frame(&self) -> bool {
        self.frame.is_some()
    }

    fn pump(&mut self, cx: &mut Context<Self>) {
        for e in self.engine.drain_events() {
            match e {
                MpvEvent::TimePos(v) => self.state.position = v,
                MpvEvent::Duration(v) => self.state.duration = v,
                MpvEvent::Buffered(v) => self.state.buffered = v,
                MpvEvent::Volume(v) => self.state.volume = v,
                MpvEvent::Muted(v) => self.state.muted = v,
                MpvEvent::Seeking(v) => {
                    if self.state.seeking && !v {
                        cx.emit(PlayerEvent::Seeked);
                    }
                    self.state.seeking = v;
                }
                MpvEvent::Paused(v) => {
                    self.state.paused = v;
                    cx.emit(PlayerEvent::Paused(v));
                }
                MpvEvent::Loaded => {
                    self.state.loaded = true;
                    cx.emit(PlayerEvent::Loaded);
                }
                MpvEvent::Ended { eof, error } => cx.emit(PlayerEvent::Ended { eof, error }),
            }
        }
        if let Some(f) = self.engine.take_frame()
            && let Some(old) = self.frame.replace(f)
        {
            self.retired.push(old);
        }
        cx.notify();
    }
}

impl Render for Player {
    fn render(&mut self, window: &mut Window, _: &mut Context<Self>) -> impl IntoElement {
        for old in self.retired.drain(..) {
            let _ = window.drop_image(old);
        }
        let frame = self.frame.clone();
        let engine = self.engine.clone();
        canvas(
            move |bounds, window, _| {
                let s = window.scale_factor();
                let w = (f32::from(bounds.size.width) * s).round() as u32;
                let h = (f32::from(bounds.size.height) * s).round() as u32;
                engine.set_viewport(w, h);
            },
            move |bounds, _, window, _| {
                let Some(frame) = frame else { return };
                let size = frame.size(0);
                let (iw, ih) = (i32::from(size.width) as f32, i32::from(size.height) as f32);
                if iw <= 0.0 || ih <= 0.0 {
                    return;
                }
                let (bw, bh) = (f32::from(bounds.size.width), f32::from(bounds.size.height));
                let scale = (bw / iw).min(bh / ih);
                let (w, h) = (iw * scale, ih * scale);
                let image_bounds = Bounds::new(
                    point(
                        bounds.origin.x + px((bw - w) / 2.0),
                        bounds.origin.y + px((bh - h) / 2.0),
                    ),
                    size_px(w, h),
                );
                let _ = window.paint_image(
                    image_bounds,
                    image_bounds,
                    Corners::default(),
                    frame,
                    0,
                    false,
                );
            },
        )
        .size_full()
    }
}

fn size_px(w: f32, h: f32) -> Size<Pixels> {
    size(px(w), px(h))
}
