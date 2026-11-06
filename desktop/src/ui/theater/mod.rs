//! The player page and its episode session, a port of
//! `internal/playback/session.go` onto the in-window player: resolve (with the
//! device-reset retry in Go), check expiry only before a load, fall back to the
//! backup URL and then one re-resolve on a load failure, autoplay with up to
//! three unavailable episodes skipped in a row, prefetch the next model, and
//! write history at start, pause, seek, stop, every 15 s, and at the end.

mod view;

use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use gpui_kit::component::slider::{SliderEvent, SliderState};
use gpui_kit::*;

use crate::player::{LoadRequest, Player, PlayerEvent};
use crate::sdk::SdkError;
use crate::sdk::types::{Episode, HistoryEntry, PlaybackResult, Stream};
use crate::ui::i18n::fill;
use crate::ui::state::{self, Nav, PlayRequest};

const MAX_UNAVAILABLE: u32 = 3;
const RECORD_EVERY: Duration = Duration::from_secs(15);
const CONTROLS_FOR: Duration = Duration::from_millis(2500);

pub enum Status {
    Resolving,
    Playing,
    /// The service gives this device no stream right now.
    Unavailable(SdkError),
    Failed(String),
    Ended,
}

pub struct Theater {
    req: PlayRequest,
    current: Episode,
    player: Entity<Player>,
    status: Status,
    result: Option<PlaybackResult>,
    stream: Option<Stream>,
    tried_backup: bool,
    refreshed: bool,
    unavailable_run: u32,
    prefetched: Option<(i32, Result<PlaybackResult, SdkError>)>,
    last_record: Instant,
    history_warned: bool,
    activity: Instant,
    menu: Option<Menu>,
    /// The user's choice; `None` follows the layout (open beside a portrait video).
    rail: Option<bool>,
    /// The side space a portrait video leaves, worked out at each render.
    room: Room,
    seek: Entity<SliderState>,
    volume: Entity<SliderState>,
    scrubbing: bool,
    focus: FocusHandle,
    resolve_task: Option<Task<()>>,
    prefetch_task: Option<Task<()>>,
    start_at: f64,
    _ticker: Task<()>,
}

/// What fits beside the video without shrinking it.
#[derive(Debug, Clone, Copy, Default, PartialEq)]
pub struct Room {
    pub rail: bool,
    pub info: bool,
}

#[derive(Clone, Copy, PartialEq)]
pub enum Menu {
    Quality,
}

impl EventEmitter<Nav> for Theater {}

impl Focusable for Theater {
    fn focus_handle(&self, _: &App) -> FocusHandle {
        self.focus.clone()
    }
}

impl Theater {
    pub fn new(
        req: PlayRequest,
        player: Entity<Player>,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Theater {
        let volume = state::ohg(cx).settings.volume;
        cx.subscribe_in(&player, window, |this, _, e: &PlayerEvent, window, cx| {
            this.on_player(e, window, cx)
        })
        .detach();
        cx.observe_in(&player, window, |this, _, window, cx| {
            this.on_tick(window, cx)
        })
        .detach();
        let seek = cx.new(|_| SliderState::new().min(0.).max(1000.).step(1.));
        cx.subscribe_in(&seek, window, |this, _, e: &SliderEvent, _, cx| {
            this.on_seek(e, cx)
        })
        .detach();
        let vol = cx.new(|_| {
            SliderState::new()
                .min(0.)
                .max(130.)
                .step(1.)
                .default_value(volume as f32)
        });
        cx.subscribe_in(&vol, window, |this, _, e: &SliderEvent, window, cx| {
            this.on_volume(e, window, cx)
        })
        .detach();
        let ticker = cx.spawn(async move |this, cx| {
            loop {
                cx.background_executor()
                    .timer(Duration::from_millis(500))
                    .await;
                if this.update(cx, |t, cx| t.on_timer(cx)).is_err() {
                    return;
                }
            }
        });
        let start = req.start.clone();
        let start_seconds = req.start_seconds;
        let mut theater = Theater {
            current: start.clone(),
            req,
            player,
            status: Status::Resolving,
            result: None,
            stream: None,
            tried_backup: false,
            refreshed: false,
            unavailable_run: 0,
            prefetched: None,
            last_record: Instant::now(),
            history_warned: false,
            activity: Instant::now(),
            menu: None,
            rail: None,
            room: Room::default(),
            seek,
            volume: vol,
            scrubbing: false,
            focus: cx.focus_handle(),
            resolve_task: None,
            prefetch_task: None,
            start_at: start_seconds,
            _ticker: ticker,
        };
        theater.start(start, start_seconds, cx);
        theater
    }

    fn autoplay(&self, cx: &App) -> bool {
        state::ohg(cx).settings.autoplay
    }

    fn next_episode(&self, from: i32) -> Option<Episode> {
        self.req
            .episodes
            .iter()
            .find(|e| e.number == from + 1)
            .cloned()
    }

    fn prev_episode(&self, from: i32) -> Option<Episode> {
        self.req
            .episodes
            .iter()
            .find(|e| e.number == from - 1)
            .cloned()
    }

    pub fn start(&mut self, ep: Episode, start_seconds: f64, cx: &mut Context<Self>) {
        if self.matches_current_playing(&ep) {
            return;
        }
        self.player.update(cx, |p, cx| p.stop(cx));
        self.current = ep.clone();
        self.status = Status::Resolving;
        self.result = None;
        self.stream = None;
        self.tried_backup = false;
        self.refreshed = false;
        self.menu = None;
        self.start_at = start_seconds;
        match self.prefetched.take() {
            Some((n, result)) if n == ep.number => self.on_resolved(result, start_seconds, cx),
            _ => self.resolve(start_seconds, cx),
        }
        if self.autoplay(cx)
            && let Some(next) = self.next_episode(ep.number)
        {
            self.prefetch(next, cx);
        }
        cx.notify();
    }

    fn matches_current_playing(&self, ep: &Episode) -> bool {
        ep.id == self.current.id && matches!(self.status, Status::Playing) && self.result.is_some()
    }

    fn resolve(&mut self, start_seconds: f64, cx: &mut Context<Self>) {
        let call = state::sdk(cx)
            .call::<PlaybackResult>("resolvePlaybackFresh", (self.current.id.clone(),));
        self.resolve_task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |t, cx| t.on_resolved(result, start_seconds, cx));
        }));
    }

    fn prefetch(&mut self, ep: Episode, cx: &mut Context<Self>) {
        let call = state::sdk(cx).call::<PlaybackResult>("resolvePlaybackFresh", (ep.id.clone(),));
        self.prefetch_task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |t, _| t.prefetched = Some((ep.number, result)));
        }));
    }

    fn on_resolved(
        &mut self,
        result: Result<PlaybackResult, SdkError>,
        start_seconds: f64,
        cx: &mut Context<Self>,
    ) {
        let result = match result {
            Ok(r) if r.playback.expired(now_unix()) && !self.refreshed => {
                self.refreshed = true;
                return self.resolve(start_seconds, cx);
            }
            Ok(r) => r,
            Err(err) => return self.on_unplayable(err, cx),
        };
        let quality = state::ohg(cx).settings.quality.clone();
        let Some(stream) = result.playback.select(&quality) else {
            return self.on_unplayable(
                SdkError {
                    code: "CONTENT_UNAVAILABLE".into(),
                    ..SdkError::local("no stream")
                },
                cx,
            );
        };
        let key = result
            .content_key
            .as_ref()
            .map(|k| k.key.clone())
            .unwrap_or_default();
        if result.playback.protected && key.is_empty() {
            return self.on_unplayable(
                SdkError {
                    code: "DRM_PROTECTED".into(),
                    ..SdkError::local("no content key")
                },
                cx,
            );
        }
        self.result = Some(result);
        self.load(stream, start_seconds, false, cx);
    }

    fn load(&mut self, stream: Stream, start: f64, backup: bool, cx: &mut Context<Self>) {
        let Some(result) = &self.result else { return };
        let req = LoadRequest {
            url: if backup {
                stream.backup_url.clone()
            } else {
                stream.url.clone()
            },
            audio_url: result.playback.audio_url.clone(),
            key: result
                .content_key
                .as_ref()
                .map(|k| k.key.clone())
                .unwrap_or_default(),
            start,
        };
        self.stream = Some(stream);
        if let Err(err) = self.player.update(cx, |p, cx| p.load(&req, cx)) {
            self.status = Status::Failed(format!("{err:#}"));
        }
        cx.notify();
    }

    /// `playback.Unavailable` skips under autoplay; device-level refusals stop
    /// with the "try later" screen; anything else is a plain failure.
    fn on_unplayable(&mut self, err: SdkError, cx: &mut Context<Self>) {
        let skippable = matches!(
            err.code.as_str(),
            "ANONYMOUS_LIMITED" | "AUTHENTICATION_REQUIRED" | "DRM_PROTECTED"
        );
        if skippable && self.autoplay(cx) {
            self.unavailable_run += 1;
            if self.unavailable_run < MAX_UNAVAILABLE
                && let Some(next) = self.next_episode(self.current.number)
            {
                cx.emit(Nav::error(fill(
                    state::tr(cx).toast_skipped,
                    self.current.number,
                )));
                return self.start(next, 0.0, cx);
            }
        }
        let device = matches!(
            err.code.as_str(),
            "ANONYMOUS_LIMITED" | "DEVICE_REJECTED" | "CHALLENGE_REQUIRED" | "DRM_PROTECTED"
        );
        self.status = if device || skippable {
            Status::Unavailable(err)
        } else {
            Status::Failed(err.describe(state::zh(cx)))
        };
        cx.notify();
    }

    fn on_player(&mut self, e: &PlayerEvent, _: &mut Window, cx: &mut Context<Self>) {
        match e {
            PlayerEvent::Loaded => {
                self.status = Status::Playing;
                self.unavailable_run = 0;
                self.record(self.start_at, false, cx);
            }
            PlayerEvent::Paused(true) | PlayerEvent::Seeked => {
                let pos = self.player.read(cx).state.position;
                self.record(pos, false, cx);
            }
            PlayerEvent::Paused(false) => {}
            PlayerEvent::Ended { eof: true, .. } => {
                self.record(0.0, true, cx);
                match self
                    .next_episode(self.current.number)
                    .filter(|_| self.autoplay(cx))
                {
                    Some(next) => self.start(next, 0.0, cx),
                    None => self.status = Status::Ended,
                }
            }
            PlayerEvent::Ended { error, .. } => {
                self.on_load_failed(error.clone().unwrap_or_default(), cx)
            }
        }
        cx.notify();
    }

    /// Backup CDN once, then one re-resolve at the last position.
    fn on_load_failed(&mut self, error: String, cx: &mut Context<Self>) {
        let pos = self.player.read(cx).state.position;
        if let Some(stream) = self.stream.clone()
            && !self.tried_backup
            && !stream.backup_url.is_empty()
        {
            self.tried_backup = true;
            return self.load(stream, pos, true, cx);
        }
        if !self.refreshed {
            self.refreshed = true;
            self.tried_backup = false;
            self.status = Status::Resolving;
            return self.resolve(pos, cx);
        }
        self.status = Status::Failed(error);
    }

    pub fn retry(&mut self, cx: &mut Context<Self>) {
        self.unavailable_run = 0;
        let ep = self.current.clone();
        self.status = Status::Ended;
        self.start(ep, 0.0, cx);
    }

    pub fn close(&mut self, cx: &mut Context<Self>) {
        if matches!(self.status, Status::Playing) {
            let pos = self.player.read(cx).state.position;
            self.record(pos, false, cx);
        }
        self.player.update(cx, |p, cx| p.stop(cx));
        cx.emit(Nav::Back);
    }

    fn on_tick(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if !self.scrubbing {
            let s = &self.player.read(cx).state;
            let v = if s.duration > 0.0 {
                (s.position / s.duration * 1000.0) as f32
            } else {
                0.0
            };
            let vol = if s.muted { 0.0 } else { s.volume as f32 };
            self.seek.update(cx, |st, cx| st.set_value(v, window, cx));
            self.volume
                .update(cx, |st, cx| st.set_value(vol, window, cx));
        }
        cx.notify();
    }

    fn on_timer(&mut self, cx: &mut Context<Self>) {
        let s = self.player.read(cx).state.clone();
        if matches!(self.status, Status::Playing)
            && !s.paused
            && self.last_record.elapsed() >= RECORD_EVERY
        {
            self.record(s.position, false, cx);
        }
        // Wake once when the controls should fade out.
        let idle = self.activity.elapsed();
        if idle >= CONTROLS_FOR && idle < CONTROLS_FOR + Duration::from_millis(600) {
            cx.notify();
        }
    }

    fn on_seek(&mut self, e: &SliderEvent, cx: &mut Context<Self>) {
        match e {
            SliderEvent::Change(_) => self.scrubbing = true,
            SliderEvent::Release(v) => {
                self.scrubbing = false;
                let duration = self.player.read(cx).state.duration;
                self.player
                    .update(cx, |p, _| p.seek_to(v.start() as f64 / 1000.0 * duration));
            }
        }
        self.poke(cx);
    }

    fn on_volume(&mut self, e: &SliderEvent, window: &mut Window, cx: &mut Context<Self>) {
        let v = match e {
            SliderEvent::Change(v) | SliderEvent::Release(v) => v.start() as f64,
        };
        self.player.update(cx, |p, _| p.set_volume(v));
        if matches!(e, SliderEvent::Release(_)) {
            let _ = state::update_settings(window, cx, |s| s.volume = v);
        }
        self.poke(cx);
    }

    fn record(&mut self, position: f64, finished: bool, cx: &mut Context<Self>) {
        self.last_record = Instant::now();
        let entry = HistoryEntry {
            series_id: self.req.series.id.clone(),
            title: self.req.series.title.clone(),
            episode: self.current.number,
            episodes: self.req.episodes.len() as i32,
            finished,
            position: if finished { 0.0 } else { position.max(0.0) },
            updated_at: String::new(),
        };
        let call = state::sdk(cx).call::<serde_json::Value>("historyRecord", [entry]);
        // Detached so the write on close outlives the theater.
        cx.spawn(async move |this, cx| match call.await {
            Ok(_) => {
                cx.update(|cx| state::reload_history(cx).detach());
            }
            Err(err) => {
                log::warn!("history: {err}");
                let _ = this.update(cx, |t, cx| {
                    if !t.history_warned {
                        t.history_warned = true;
                        cx.emit(Nav::error(state::tr(cx).toast_history));
                    }
                });
            }
        })
        .detach();
    }

    /// Mouse or key activity: show the controls for a while.
    pub fn poke(&mut self, cx: &mut Context<Self>) {
        let hidden = self.activity.elapsed() >= CONTROLS_FOR;
        self.activity = Instant::now();
        if hidden {
            cx.notify();
        }
    }

    fn controls_visible(&self, cx: &App) -> bool {
        let s = &self.player.read(cx).state;
        !matches!(self.status, Status::Playing)
            || s.paused
            || self.menu.is_some()
            || self.scrubbing
            || self.activity.elapsed() < CONTROLS_FOR
    }

    pub fn key(&mut self, key: &str, window: &mut Window, cx: &mut Context<Self>) -> bool {
        let step = |p: &mut Player, by: f64| p.set_volume((p.state.volume + by).clamp(0.0, 130.0));
        match key {
            "space" => self.player.update(cx, |p, _| p.toggle_pause()),
            "left" => self.player.update(cx, |p, _| p.seek_by(-5.0)),
            "right" => self.player.update(cx, |p, _| p.seek_by(5.0)),
            "up" => self.player.update(cx, |p, _| step(p, 5.0)),
            "down" => self.player.update(cx, |p, _| step(p, -5.0)),
            "m" => self.player.update(cx, |p, _| p.toggle_mute()),
            "f" => window.toggle_fullscreen(),
            "n" => {
                if let Some(next) = self.next_episode(self.current.number) {
                    self.jump(next, cx);
                }
            }
            "p" => {
                if let Some(prev) = self.prev_episode(self.current.number) {
                    self.jump(prev, cx);
                }
            }
            "escape" => {
                if self.menu.is_some() {
                    self.menu = None;
                } else if window.is_fullscreen() {
                    window.toggle_fullscreen();
                } else {
                    self.close(cx);
                }
            }
            _ => return false,
        }
        self.poke(cx);
        true
    }

    /// A user jump records where the current episode was left.
    pub fn jump(&mut self, ep: Episode, cx: &mut Context<Self>) {
        if !ep.playable {
            cx.emit(Nav::error(state::tr(cx).toast_locked));
            return;
        }
        if matches!(self.status, Status::Playing) {
            let pos = self.player.read(cx).state.position;
            self.record(pos, false, cx);
        }
        self.unavailable_run = 0;
        self.start(ep, 0.0, cx);
    }

    pub fn set_quality(&mut self, quality: String, window: &mut Window, cx: &mut Context<Self>) {
        self.menu = None;
        let _ = state::update_settings(window, cx, |s| s.quality = quality.clone());
        let Some(result) = &self.result else { return };
        let Some(stream) = result.playback.select(&quality) else {
            return;
        };
        if self.stream.as_ref().is_some_and(|s| s.url == stream.url) {
            return;
        }
        let pos = self.player.read(cx).state.position;
        self.tried_backup = false;
        self.load(stream, pos, false, cx);
    }
}

fn now_unix() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .expect("system clock before 1970")
}
