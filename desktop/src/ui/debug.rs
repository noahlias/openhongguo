//! The debug drawer (F12): every call across the Go FFI bridge, libmpv's
//! decoder and render numbers, and the window. It samples twice a second so
//! the per-frame redraws while playing cost nothing extra.

use std::time::{Duration, Instant};

use gpui_kit::assets::IconName;
use gpui_kit::component::Theme;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::icons::icon;
use super::state;
use super::theme::{Palette, palette};
use super::widgets;
use crate::player::Player;
use crate::player::engine::Stats;
use crate::sdk::trace::{CallRecord, Outcome, Snapshot};

actions!(ohg, [ToggleDebug]);

const SAMPLE_EVERY: Duration = Duration::from_millis(500);
const SHOWN_CALLS: usize = 60;
pub const PANEL_W: f32 = 420.0;

pub struct DebugPanel {
    pub player: Option<Entity<Player>>,
    calls: Snapshot,
    stats: Option<Stats>,
    rates: Option<Rates>,
    last: Option<(Instant, u64, f64)>,
    ping: Option<String>,
    ping_task: Option<Task<()>>,
    _ticker: Task<()>,
}

#[derive(Clone, Copy)]
struct Rates {
    fps: f64,
    /// Percent of one core, the whole process.
    cpu: f64,
}

impl DebugPanel {
    pub fn new(player: Option<Entity<Player>>, cx: &mut Context<Self>) -> DebugPanel {
        let ticker = cx.spawn(async move |this, cx| {
            loop {
                if this.update(cx, |d, cx| d.sample(cx)).is_err() {
                    return;
                }
                cx.background_executor().timer(SAMPLE_EVERY).await;
            }
        });
        DebugPanel {
            player,
            calls: Snapshot::default(),
            stats: None,
            rates: None,
            last: None,
            ping: None,
            ping_task: None,
            _ticker: ticker,
        }
    }

    fn sample(&mut self, cx: &mut Context<Self>) {
        self.calls = state::sdk(cx).trace().snapshot();
        self.stats = self.player.as_ref().map(|p| p.read(cx).stats());
        let now = Instant::now();
        let frames = self.stats.as_ref().map_or(0, |s| s.rendered);
        let cpu = cpu_seconds();
        if let Some((at, last_frames, last_cpu)) = self.last {
            let secs = now.duration_since(at).as_secs_f64();
            self.rates = Some(Rates {
                fps: frames.saturating_sub(last_frames) as f64 / secs,
                cpu: (cpu - last_cpu) / secs * 100.0,
            });
        }
        self.last = Some((now, frames, cpu));
        cx.notify();
    }

    /// A round trip through the bridge with a call that reads only the local
    /// history file, so the time is the FFI and JSON cost plus one file read.
    fn ping(&mut self, cx: &mut Context<Self>) {
        let call = state::sdk(cx).call::<serde_json::Value>("historyList", ());
        let began = Instant::now();
        self.ping = Some("…".into());
        self.ping_task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let elapsed = began.elapsed();
            let _ = this.update(cx, |d, cx| {
                d.ping = Some(match result {
                    Ok(v) => format!(
                        "ok · {} · {} entries",
                        ms(elapsed),
                        v.as_array().map_or(0, Vec::len)
                    ),
                    Err(e) => format!("{} · {}", e.code, ms(elapsed)),
                });
                d.sample(cx);
            });
        }));
        cx.notify();
    }
}

impl Render for DebugPanel {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let p = palette(cx);
        let mono = Theme::global(cx).mono_font_family.clone();
        let sdk = state::sdk(cx);
        let c = &self.calls;
        let finished: Vec<&CallRecord> = c.recent.iter().filter(|r| r.elapsed.is_some()).collect();
        let avg = (!finished.is_empty()).then(|| {
            finished
                .iter()
                .map(|r| r.elapsed.unwrap())
                .sum::<Duration>()
                / finished.len() as u32
        });
        let slowest = finished.iter().max_by_key(|r| r.elapsed);
        let bridge = section("FFI bridge · Go c-archive", p)
            .child(kv("library", "libohg.a (static, cgo)", p))
            .child(kv("handle", format!("#{}", sdk.handle()), p))
            .child(kv(
                "calls",
                format!(
                    "{} total · {} in flight · {} errors",
                    c.total, c.in_flight, c.errors
                ),
                p,
            ))
            .child(kv(
                "avg latency",
                avg.map(ms).unwrap_or_else(|| "—".into()),
                p,
            ))
            .when_some(slowest, |d, r| {
                d.child(kv(
                    "slowest",
                    format!("{} · {}", r.method, ms(r.elapsed.unwrap())),
                    p,
                ))
            })
            .child(
                div()
                    .pt(px(6.))
                    .flex()
                    .items_center()
                    .gap(px(10.))
                    .child(
                        widgets::ghost_button("dbg-ping", "Ping", Some(IconName::RefreshCw), p)
                            .on_click(cx.listener(|this, _, _, cx| this.ping(cx))),
                    )
                    .when_some(self.ping.clone(), |d, s| {
                        d.child(div().font_family(mono.clone()).text_color(p.dim).child(s))
                    }),
            );
        let calls = section("Recent calls", p).children(
            c.recent
                .iter()
                .rev()
                .take(SHOWN_CALLS)
                .map(|r| call_row(r, mono.clone(), p)),
        );
        let player = section("Player · libmpv", p).map(|d| match &self.stats {
            None => d.child(div().text_color(p.dim).child("not started")),
            Some(s) if s.video_codec.is_none() => d.child(
                div()
                    .text_color(p.dim)
                    .child(format!("idle · {} frames drawn so far", s.rendered)),
            ),
            Some(s) => d
                .child(kv("hwdec", opt(&s.hwdec), p))
                .child(kv(
                    "codec",
                    format!("{} / {}", opt(&s.video_codec), opt(&s.audio_codec)),
                    p,
                ))
                .child(kv("source", dims(s.source), p))
                .child(kv("rendered at", dims(s.output), p))
                .child(kv(
                    "fps",
                    format!(
                        "file {} · decoded {} · drawn {}",
                        num(s.container_fps, 2),
                        num(s.estimated_fps, 2),
                        self.rates.map_or("—".into(), |r| format!("{:.1}", r.fps))
                    ),
                    p,
                ))
                .child(kv(
                    "dropped",
                    format!(
                        "vo {} · decoder {}",
                        s.dropped.map_or("—".into(), |v| v.to_string()),
                        s.decoder_dropped.map_or("—".into(), |v| v.to_string())
                    ),
                    p,
                ))
                .child(kv("cache", num(s.cache_seconds, 1) + " s", p))
                .child(kv(
                    "bitrate",
                    s.video_bitrate
                        .map_or("—".into(), |b| format!("{:.0} kb/s", b / 1000.0)),
                    p,
                ))
                .child(kv("frames drawn", s.rendered.to_string(), p)),
        });
        let size = window.viewport_size();
        let inspector = if cfg!(any(feature = "inspector", debug_assertions)) {
            "Ctrl+Shift+I"
        } else {
            "build with --features inspector"
        };
        let ui = section("GPUI", p)
            .child(kv(
                "window",
                format!(
                    "{:.0}×{:.0} @{}x",
                    f32::from(size.width),
                    f32::from(size.height),
                    window.scale_factor()
                ),
                p,
            ))
            .child(kv(
                "process CPU",
                self.rates
                    .map_or("—".into(), |r| format!("{:.0}% of one core", r.cpu)),
                p,
            ))
            .child(kv(
                "build",
                if cfg!(debug_assertions) {
                    "debug"
                } else {
                    "release"
                },
                p,
            ))
            .child(kv("inspector", inspector, p));
        div()
            .id("debug")
            .occlude()
            .size_full()
            .bg(p.raised)
            .border_l_1()
            .border_color(p.line)
            .text_xs()
            .text_color(p.fg)
            .flex()
            .flex_col()
            .child(
                div()
                    .px(px(18.))
                    .py(px(14.))
                    .flex()
                    .items_center()
                    .justify_between()
                    .border_b_1()
                    .border_color(p.line)
                    .child(
                        div()
                            .flex()
                            .items_center()
                            .gap(px(8.))
                            .child(widgets::subtitle("Debug", p))
                            .child(widgets::pill("F12", p)),
                    )
                    .child(
                        div()
                            .id("dbg-close")
                            .size(px(28.))
                            .flex()
                            .items_center()
                            .justify_center()
                            .rounded_full()
                            .cursor_pointer()
                            .hover(|s| s.bg(p.hover))
                            .child(icon(IconName::Close, 15., p.dim))
                            .on_click(|_, window, cx| {
                                window.dispatch_action(Box::new(ToggleDebug), cx)
                            }),
                    ),
            )
            .child(
                div()
                    .id("debug-body")
                    .flex_1()
                    .overflow_y_scroll()
                    .px(px(18.))
                    .pb(px(24.))
                    .child(bridge)
                    .child(player)
                    .child(ui)
                    .child(calls),
            )
    }
}

fn section(title: &'static str, p: Palette) -> Div {
    div().pt(px(16.)).flex().flex_col().gap(px(5.)).child(
        div()
            .pb(px(4.))
            .text_xs()
            .font_weight(FontWeight::SEMIBOLD)
            .text_color(p.accent)
            .child(title),
    )
}

fn kv(key: &'static str, value: impl Into<SharedString>, p: Palette) -> Div {
    div()
        .flex()
        .gap(px(12.))
        .child(div().w(px(96.)).flex_none().text_color(p.dim).child(key))
        .child(div().flex_1().min_w_0().child(value.into()))
}

fn call_row(r: &CallRecord, mono: SharedString, p: Palette) -> Div {
    let (label, color) = match &r.outcome {
        Outcome::Pending => ("…".to_string(), p.orange),
        Outcome::Ok(bytes) => (size(*bytes), p.green),
        Outcome::Err(code) => (code.clone(), p.accent),
        Outcome::Cancelled => ("cancelled".to_string(), p.faint),
    };
    div()
        .py(px(5.))
        .border_b_1()
        .border_color(p.line)
        .flex()
        .flex_col()
        .gap(px(2.))
        .child(
            div()
                .flex()
                .items_center()
                .gap(px(8.))
                .child(
                    div()
                        .flex_1()
                        .min_w_0()
                        .truncate()
                        .font_family(mono.clone())
                        .child(r.method.clone()),
                )
                .child(div().text_color(color).child(label))
                .child(
                    div()
                        .w(px(64.))
                        .flex_none()
                        .text_right()
                        .text_color(p.dim)
                        .child(r.elapsed.map(ms).unwrap_or_else(|| "…".into())),
                ),
        )
        .child(
            div()
                .truncate()
                .font_family(mono)
                .text_color(p.faint)
                .child(r.args.clone()),
        )
}

/// User plus system CPU time this process has used.
fn cpu_seconds() -> f64 {
    let mut usage = std::mem::MaybeUninit::<libc::rusage>::uninit();
    // SAFETY: getrusage fills the struct for RUSAGE_SELF; it fails only on a bad `who`.
    let usage = unsafe {
        assert_eq!(libc::getrusage(libc::RUSAGE_SELF, usage.as_mut_ptr()), 0);
        usage.assume_init()
    };
    let tv = |t: libc::timeval| t.tv_sec as f64 + t.tv_usec as f64 / 1e6;
    tv(usage.ru_utime) + tv(usage.ru_stime)
}

fn ms(d: Duration) -> String {
    format!("{:.1} ms", d.as_secs_f64() * 1000.0)
}

fn size(bytes: usize) -> String {
    if bytes < 1024 {
        format!("{bytes} B")
    } else {
        format!("{:.1} KB", bytes as f64 / 1024.0)
    }
}

fn opt(v: &Option<String>) -> String {
    v.clone()
        .filter(|s| !s.is_empty())
        .unwrap_or_else(|| "—".into())
}

fn num(v: Option<f64>, digits: usize) -> String {
    v.map_or("—".into(), |v| format!("{v:.digits$}"))
}

fn dims((w, h): (u32, u32)) -> String {
    if w == 0 {
        "—".into()
    } else {
        format!("{w}×{h}")
    }
}
