//! The theater's look: the frame, a top and bottom bar that fade after
//! inactivity, the status screens, and the episode rail beside the video.
//! Always dark, whatever the app theme.

use std::time::Duration;

use gpui_kit::assets::IconName;
use gpui_kit::component::slider::Slider;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::controls::{control, pill_button, speed_label};
use super::rail::RAIL_W;
use super::{Menu, Status, Theater};
use crate::ui::format;
use crate::ui::i18n::{Tr, fill};
use crate::ui::icons::icon;
use crate::ui::layout;
use crate::ui::motion;
use crate::ui::state;
use crate::ui::theme::Palette;
use crate::ui::widgets;

/// Narrower than this beside the rail, the video keeps the width and the rail
/// floats over it.
const STAGE_MIN: f32 = 480.0;
/// A stage narrower than this drops the volume slider and the autoplay pill.
const NARROW: f32 = 640.0;

/// The width a portrait video takes at full height; `None` for landscape or
/// before the size is known.
fn portrait_width(video: (u32, u32), height: f32) -> Option<f32> {
    let (vw, vh) = video;
    (vw > 0 && vh > vw).then(|| height * vw as f32 / vh as f32)
}

impl Render for Theater {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let p = Palette::dark();
        let t = state::tr(cx);
        let show = self.controls_visible(cx);
        let width = layout::usable_width(window, cx);
        let height = f32::from(window.viewport_size().height);
        let fit = portrait_width(self.player.read(cx).video_size(), height);
        self.room = fit.is_some_and(|w| width - w >= RAIL_W);
        let rail = self.rail_open();
        let float_rail = rail && width - RAIL_W < STAGE_MIN;
        // Docked beside a portrait video, the stage is just the video's width,
        // so the rail sits against it and the two centre together.
        let hug = fit.filter(|w| rail && !float_rail && w + RAIL_W <= width);
        let stage_width = match hug {
            Some(w) => w,
            None if rail && !float_rail => width - RAIL_W,
            None => width,
        };
        let narrow = stage_width < NARROW;
        let stage = div()
            .relative()
            .map(|d| match hug {
                Some(w) => d.flex_none().w(px(w)),
                None => d.flex_1(),
            })
            .min_w_0()
            .h_full()
            .overflow_hidden()
            .child(div().absolute().inset_0().child(self.player.clone()))
            .child(
                div()
                    .id("video-hit")
                    .absolute()
                    .inset_0()
                    .on_click(cx.listener(|this, e: &ClickEvent, window, cx| {
                        if this.menu.take().is_some() {
                            cx.notify();
                        } else if e.click_count() >= 2 {
                            window.toggle_fullscreen();
                        } else if matches!(this.status, Status::Playing) {
                            this.player.update(cx, |pl, _| pl.toggle_pause());
                        }
                        this.poke(cx);
                    })),
            )
            .child(self.status_screen(t, p, cx))
            .when(show, |d| {
                d.child(motion::enter(
                    self.top_bar(t, p, cx),
                    "top-bar",
                    180,
                    Duration::ZERO,
                    0.,
                ))
            })
            .when(float_rail, |d| {
                d.child(
                    div()
                        .absolute()
                        .top_0()
                        .bottom_0()
                        .right_0()
                        .child(self.rail_panel(t, p, cx)),
                )
            })
            .when(show, |d| {
                d.child(motion::enter(
                    self.bottom_bar(t, p, narrow, cx),
                    "bottom-bar",
                    180,
                    Duration::ZERO,
                    0.,
                ))
            });
        div()
            .id("theater")
            .occlude()
            .track_focus(&self.focus)
            .size_full()
            .flex()
            .justify_center()
            .overflow_hidden()
            .bg(hsla(0., 0., 0., 1.))
            .text_color(p.fg)
            .on_key_down(cx.listener(|this, e: &KeyDownEvent, window, cx| {
                if this.key(e.keystroke.key.as_str(), window, cx) {
                    cx.stop_propagation();
                }
            }))
            .on_mouse_move(cx.listener(|this, _: &MouseMoveEvent, _, cx| this.poke(cx)))
            .child(stage)
            .when(rail && !float_rail, |d| d.child(self.rail_panel(t, p, cx)))
    }
}

impl Theater {
    fn rail_open(&self) -> bool {
        self.rail.unwrap_or(self.room)
    }

    fn status_screen(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> AnyElement {
        let loaded = self.player.read(cx).state.loaded;
        let body = match &self.status {
            Status::Playing if loaded => return div().into_any_element(),
            Status::Resolving | Status::Playing => div()
                .flex()
                .flex_col()
                .items_center()
                .gap(px(12.))
                .child(icon(IconName::LoaderCircle, 30., p.dim))
                .child(div().text_sm().text_color(p.dim).child(t.loading)),
            Status::Unavailable(err) => self.message(
                IconName::Lock,
                t.unavailable_title,
                format!("{}\n{}", t.unavailable_body, err.describe(state::zh(cx))),
                p,
                cx,
            ),
            Status::Failed(msg) => self.message(IconName::TriangleAlert, "", msg.clone(), p, cx),
            Status::Ended => self.message(IconName::Check, t.finished, String::new(), p, cx),
        };
        div()
            .absolute()
            .inset_0()
            .flex()
            .items_center()
            .justify_center()
            .child(body)
            .into_any_element()
    }

    fn message(
        &self,
        ic: IconName,
        title: &'static str,
        body: String,
        p: Palette,
        cx: &mut Context<Self>,
    ) -> Div {
        let t = state::tr(cx);
        let next = self.next_episode(self.current.number);
        div()
            .occlude()
            .max_w(px(440.))
            .px(px(28.))
            .py(px(26.))
            .rounded(px(18.))
            .bg(p.raised.opacity(0.92))
            .border_1()
            .border_color(p.line)
            .flex()
            .flex_col()
            .items_center()
            .gap(px(12.))
            .child(icon(ic, 30., p.accent))
            .when(!title.is_empty(), |d| {
                d.child(
                    div()
                        .text_lg()
                        .font_weight(FontWeight::SEMIBOLD)
                        .child(title),
                )
            })
            .when(!body.is_empty(), |d| {
                d.child(
                    div()
                        .text_sm()
                        .text_color(p.dim)
                        .text_center()
                        .children(body.lines().map(|l| div().child(l.to_string()))),
                )
            })
            .child(
                div()
                    .pt(px(6.))
                    .flex()
                    .gap(px(10.))
                    .child(
                        widgets::primary_button("t-retry", t.retry, Some(IconName::RefreshCw), p)
                            .on_click(cx.listener(|this, _, _, cx| this.retry(cx))),
                    )
                    .when_some(next, |d, next| {
                        d.child(
                            widgets::ghost_button(
                                "t-next",
                                t.up_next,
                                Some(IconName::SkipForward),
                                p,
                            )
                            .on_click(
                                cx.listener(move |this, _, _, cx| this.jump(next.clone(), cx)),
                            ),
                        )
                    })
                    .child(
                        widgets::ghost_button("t-back", t.back, None, p)
                            .on_click(cx.listener(|this, _, _, cx| this.close(cx))),
                    ),
            )
    }

    fn top_bar(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> Div {
        let mut subtitle = fill(t.episode_n, self.current.number);
        if !self.current.title.is_empty() && self.current.title != self.req.series.title {
            subtitle = format!("{subtitle} · {}", self.current.title);
        }
        div()
            .occlude()
            .absolute()
            .top_0()
            .left_0()
            .right_0()
            .px(px(18.))
            .pt(px(14.))
            .pb(px(36.))
            .flex()
            .items_center()
            .gap(px(12.))
            .bg(linear_gradient(
                180.,
                linear_color_stop(hsla(0., 0., 0., 0.72), 0.),
                linear_color_stop(hsla(0., 0., 0., 0.), 1.),
            ))
            .child(
                control("t-close", IconName::ArrowLeft, p)
                    .on_click(cx.listener(|this, _, _, cx| this.close(cx))),
            )
            .child(
                div()
                    .flex()
                    .flex_col()
                    .min_w_0()
                    .child(
                        div()
                            .text_base()
                            .font_weight(FontWeight::SEMIBOLD)
                            .truncate()
                            .child(self.req.series.title.clone()),
                    )
                    .child(div().text_xs().text_color(p.dim).child(subtitle)),
            )
    }

    fn bottom_bar(&self, t: &'static Tr, p: Palette, narrow: bool, cx: &mut Context<Self>) -> Div {
        let s = self.player.read(cx).state.clone();
        let autoplay = self.autoplay(cx);
        let speed = self.speed(cx);
        let has_prev = self.prev_episode(self.current.number).is_some();
        let has_next = self.next_episode(self.current.number).is_some();
        let quality = self
            .stream
            .as_ref()
            .map(|s| s.label())
            .unwrap_or_else(|| t.quality.to_string());
        let speed_text = if speed == 1.0 {
            t.speed.to_string()
        } else {
            speed_label(speed)
        };
        let volume_icon = if s.muted || s.volume == 0.0 {
            IconName::VolumeX
        } else if s.volume < 50.0 {
            IconName::Volume1
        } else {
            IconName::Volume2
        };
        let left = div()
            .flex()
            .items_center()
            .gap(px(4.))
            .child(
                control("t-prev", IconName::SkipBack, p)
                    .when(!has_prev, |b| b.opacity(0.35))
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.key("p", window, cx);
                    })),
            )
            .child(
                control(
                    "t-play",
                    if s.paused {
                        IconName::Play
                    } else {
                        IconName::Pause
                    },
                    p,
                )
                .on_click(
                    cx.listener(|this, _, _, cx| this.player.update(cx, |pl, _| pl.toggle_pause())),
                ),
            )
            .child(
                control("t-next", IconName::SkipForward, p)
                    .when(!has_next, |b| b.opacity(0.35))
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.key("n", window, cx);
                    })),
            )
            .child(div().pl(px(8.)).text_xs().text_color(p.dim).child(format!(
                "{} / {}",
                format::clock(s.position),
                format::clock(s.duration)
            )))
            .child(
                div()
                    .pl(px(10.))
                    .flex()
                    .items_center()
                    .gap(px(6.))
                    .child(control("t-mute", volume_icon, p).on_click(cx.listener(
                        |this, _, _, cx| this.player.update(cx, |pl, _| pl.toggle_mute()),
                    )))
                    .when(!narrow, |d| {
                        d.child(div().w(px(96.)).child(Slider::new(&self.volume)))
                    }),
            );
        let right = div()
            .flex()
            .items_center()
            .gap(px(6.))
            .when(!narrow, |d| {
                d.child(
                    pill_button(
                        "t-autoplay",
                        t.autoplay,
                        Some(IconName::Repeat),
                        autoplay,
                        p,
                    )
                    .on_click(cx.listener(|this, _, window, cx| this.toggle_autoplay(window, cx))),
                )
            })
            .child(self.menu_pill(Menu::Speed, speed_text.into(), speed != 1.0, p, cx))
            .child(self.menu_pill(Menu::Quality, quality.into(), false, p, cx))
            .child(
                control("t-rail", IconName::ListVideo, p)
                    .when(self.rail_open(), |b| b.bg(p.accent_soft))
                    .on_click(cx.listener(|this, _, _, cx| {
                        this.rail = Some(!this.rail_open());
                        cx.notify();
                    })),
            )
            .child(
                control("t-full", IconName::Maximize, p)
                    .on_click(cx.listener(|_, _, window, _| window.toggle_fullscreen())),
            );
        div()
            .occlude()
            .absolute()
            .bottom_0()
            .left_0()
            .right_0()
            .px(px(18.))
            .pt(px(48.))
            .pb(px(12.))
            .flex()
            .flex_col()
            .gap(px(6.))
            .bg(linear_gradient(
                0.,
                linear_color_stop(hsla(0., 0., 0., 0.8), 0.),
                linear_color_stop(hsla(0., 0., 0., 0.), 1.),
            ))
            .child(div().px(px(6.)).child(Slider::new(&self.seek)))
            .child(
                div()
                    .flex()
                    .items_center()
                    .justify_between()
                    .child(left)
                    .child(right),
            )
    }

    fn toggle_autoplay(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if let Err(err) = state::update_settings(window, cx, |s| s.autoplay = !s.autoplay) {
            cx.emit(state::Nav::error(format!("{err:#}")));
        }
        self.poke(cx);
    }
}

#[cfg(test)]
mod tests {
    use super::portrait_width;

    #[test]
    fn only_a_known_portrait_video_has_a_width() {
        assert_eq!(portrait_width((720, 1280), 1440.), Some(810.));
        assert_eq!(portrait_width((1920, 1080), 1440.), None);
        assert_eq!(portrait_width((0, 0), 1440.), None);
    }
}
