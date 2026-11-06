//! The theater's look: the frame, a top and bottom bar that fade after
//! inactivity, the quality menu, the status screens, and beside the video the
//! episode rail and, for a portrait short in a wide window, the series info.
//! Always dark, whatever the app theme.

use std::time::Duration;

use gpui_kit::assets::IconName;
use gpui_kit::component::slider::Slider;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::{Menu, Room, Status, Theater};
use crate::sdk::types::Episode;
use crate::ui::format;
use crate::ui::i18n::{Tr, fill};
use crate::ui::icons::icon;
use crate::ui::layout;
use crate::ui::motion;
use crate::ui::state;
use crate::ui::theme::Palette;
use crate::ui::widgets;

const RAIL_W: f32 = 312.0;
const INFO_W: f32 = 340.0;
const MENU_BOTTOM: f32 = 64.0;
/// Narrower than this beside the rail, the video keeps the width and the rail
/// floats over it.
const STAGE_MIN: f32 = 480.0;
/// Below this the bottom bar drops the volume slider and the autoplay pill.
const NARROW: f32 = 640.0;

/// The width a portrait video leaves free at full height, and what fits in it.
fn room(video: (u32, u32), area: Size<Pixels>) -> Room {
    let (vw, vh) = video;
    if vw == 0 || vh <= vw {
        return Room::default();
    }
    let (w, h) = (f32::from(area.width), f32::from(area.height));
    let free = w - h * vw as f32 / vh as f32;
    Room {
        rail: free >= RAIL_W,
        info: free >= RAIL_W + INFO_W,
    }
}

impl Render for Theater {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let p = Palette::dark();
        let t = state::tr(cx);
        let show = self.controls_visible(cx);
        let area = size(
            px(layout::usable_width(window, cx)),
            window.viewport_size().height,
        );
        self.room = room(self.player.read(cx).video_size(), area);
        let rail = self.rail_open();
        let width = f32::from(area.width);
        let float_rail = rail && width - RAIL_W < STAGE_MIN;
        let narrow = width < NARROW;
        let stage = div()
            .relative()
            .flex_1()
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
            })
            .when(show && self.menu == Some(Menu::Quality), |d| {
                d.child(self.quality_menu(p, cx).with_animation(
                    "quality-menu",
                    Animation::new(Duration::from_millis(180)).with_easing(ease_out_quint()),
                    |el, t| el.opacity(t).bottom(px(MENU_BOTTOM - 10. * (1. - t))),
                ))
            });
        div()
            .id("theater")
            .occlude()
            .track_focus(&self.focus)
            .size_full()
            .flex()
            .overflow_hidden()
            .bg(hsla(0., 0., 0., 1.))
            .text_color(p.fg)
            .on_key_down(cx.listener(|this, e: &KeyDownEvent, window, cx| {
                if this.key(e.keystroke.key.as_str(), window, cx) {
                    cx.stop_propagation();
                }
            }))
            .on_mouse_move(cx.listener(|this, _: &MouseMoveEvent, _, cx| this.poke(cx)))
            .when(self.room.info, |d| d.child(self.info_panel(t, p, cx)))
            .child(stage)
            .when(rail && !float_rail, |d| d.child(self.rail_panel(t, p, cx)))
    }
}

impl Theater {
    fn rail_open(&self) -> bool {
        self.rail.unwrap_or(self.room.rail)
    }

    fn info_panel(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> Stateful<Div> {
        let series = &self.req.series;
        let cover = state::cover(&series.cover_url, cx);
        let total = self.req.episodes.len();
        let mut line = fill(t.episode_n, self.current.number);
        if total > 0 {
            line = format!("{line} / {}", fill(t.episodes, total));
        }
        let next = self.next_episode(self.current.number);
        div()
            .id("t-info")
            .occlude()
            .w(px(INFO_W))
            .flex_none()
            .h_full()
            .overflow_y_scroll()
            .bg(p.sidebar)
            .border_r_1()
            .border_color(p.line)
            .px(px(24.))
            .pt(px(72.))
            .pb(px(24.))
            .flex()
            .flex_col()
            .gap(px(14.))
            .child(
                div()
                    .w(px(150.))
                    .aspect_ratio(2. / 3.)
                    .rounded(px(14.))
                    .overflow_hidden()
                    .bg(p.raised)
                    .shadow_lg()
                    .when_some(cover, |d, c| {
                        d.child(
                            img(ImageSource::Render(c))
                                .size_full()
                                .object_fit(ObjectFit::Cover),
                        )
                    }),
            )
            .child(
                div()
                    .text_lg()
                    .font_weight(FontWeight::BOLD)
                    .line_clamp(3)
                    .child(series.title.clone()),
            )
            .child(div().text_sm().text_color(p.accent).child(line))
            .when(!series.description.is_empty(), |d| {
                d.child(
                    div()
                        .text_sm()
                        .text_color(p.dim)
                        .line_clamp(12)
                        .child(series.description.clone()),
                )
            })
            .when_some(next, |d, next| {
                let label = format!("{} · {}", t.up_next, fill(t.episode_n, next.number));
                d.child(
                    div().pt(px(6.)).flex().child(
                        widgets::ghost_button("t-info-next", label, Some(IconName::SkipForward), p)
                            .on_click(
                                cx.listener(move |this, _, _, cx| this.jump(next.clone(), cx)),
                            ),
                    ),
                )
            })
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
        let has_prev = self.prev_episode(self.current.number).is_some();
        let has_next = self.next_episode(self.current.number).is_some();
        let quality = self
            .stream
            .as_ref()
            .map(|s| s.label())
            .unwrap_or_else(|| t.quality.to_string());
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
            .child(
                pill_button(
                    "t-quality",
                    quality,
                    None,
                    self.menu == Some(Menu::Quality),
                    p,
                )
                .on_click(cx.listener(|this, _, _, cx| {
                    this.menu = if this.menu == Some(Menu::Quality) {
                        None
                    } else {
                        Some(Menu::Quality)
                    };
                    cx.notify();
                })),
            )
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

    fn quality_menu(&self, p: Palette, cx: &mut Context<Self>) -> Div {
        let streams = self
            .result
            .as_ref()
            .map(|r| r.playback.usable())
            .unwrap_or_default();
        let current = self
            .stream
            .as_ref()
            .map(|s| s.url.clone())
            .unwrap_or_default();
        div()
            .occlude()
            .absolute()
            .bottom(px(MENU_BOTTOM))
            .right(px(110.))
            .min_w(px(150.))
            .p(px(6.))
            .rounded(px(12.))
            .bg(p.raised.opacity(0.96))
            .border_1()
            .border_color(p.line)
            .flex()
            .flex_col()
            .children(streams.into_iter().rev().enumerate().map(|(i, s)| {
                let on = s.url == current;
                let key = if s.definition.is_empty() {
                    s.quality.clone()
                } else {
                    s.definition.clone()
                };
                let detail = if s.codec.is_empty() {
                    String::new()
                } else {
                    s.codec.to_uppercase()
                };
                div()
                    .id(("t-q", i))
                    .px(px(12.))
                    .h(px(32.))
                    .flex()
                    .items_center()
                    .justify_between()
                    .gap(px(16.))
                    .rounded(px(8.))
                    .cursor_pointer()
                    .text_sm()
                    .hover(|d| d.bg(p.hover))
                    .when(on, |d| {
                        d.text_color(p.accent).font_weight(FontWeight::SEMIBOLD)
                    })
                    .child(s.label())
                    .child(div().text_xs().text_color(p.faint).child(detail))
                    .on_click(cx.listener(move |this, _, window, cx| {
                        this.set_quality(key.clone(), window, cx)
                    }))
            }))
    }

    fn rail_panel(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> Div {
        let watched_to = state::ohg(cx)
            .history
            .get(&self.req.series.id)
            .map(|e| e.episode)
            .unwrap_or(0);
        let tiles: Vec<_> = self
            .req
            .episodes
            .iter()
            .map(|ep| {
                rail_tile(
                    ep,
                    ep.number == self.current.number,
                    ep.number < watched_to,
                    p,
                    cx,
                )
            })
            .collect();
        div()
            .occlude()
            .flex_none()
            .h_full()
            .w(px(RAIL_W))
            .bg(p.sidebar)
            .border_l_1()
            .border_color(p.line)
            .flex()
            .flex_col()
            .child(
                div()
                    .px(px(18.))
                    .pt(px(18.))
                    .pb(px(12.))
                    .flex()
                    .items_center()
                    .justify_between()
                    .child(widgets::subtitle(t.episode_list, p))
                    .child(
                        div()
                            .text_xs()
                            .text_color(p.dim)
                            .child(fill(t.episodes, self.req.episodes.len())),
                    ),
            )
            .child(
                div()
                    .id("t-rail-list")
                    .flex_1()
                    .overflow_y_scroll()
                    .px(px(14.))
                    .pb(px(18.))
                    .child(div().flex().flex_wrap().gap(px(8.)).children(tiles)),
            )
    }

    fn toggle_autoplay(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if let Err(err) = state::update_settings(window, cx, |s| s.autoplay = !s.autoplay) {
            cx.emit(state::Nav::error(format!("{err:#}")));
        }
        self.poke(cx);
    }
}

fn rail_tile(
    ep: &Episode,
    current: bool,
    watched: bool,
    p: Palette,
    cx: &mut Context<Theater>,
) -> Stateful<Div> {
    let target = ep.clone();
    div()
        .id(("t-ep", ep.number as usize))
        .w(px(48.))
        .h(px(38.))
        .flex()
        .items_center()
        .justify_center()
        .rounded(px(9.))
        .cursor_pointer()
        .text_sm()
        .border_1()
        .when(current, |d| {
            d.bg(p.accent)
                .border_color(p.accent)
                .text_color(p.accent_fg)
                .font_weight(FontWeight::SEMIBOLD)
        })
        .when(!current && watched, |d| {
            d.bg(p.raised).border_color(p.line).text_color(p.faint)
        })
        .when(!current && !watched, |d| {
            d.bg(p.raised)
                .border_color(p.line)
                .text_color(p.fg)
                .hover(|s| s.border_color(p.dim))
        })
        .when(!ep.playable, |d| {
            d.child(icon(IconName::Lock, 13., p.faint))
        })
        .when(ep.playable, |d| d.child(ep.number.to_string()))
        .on_click(cx.listener(move |this, _, _, cx| this.jump(target.clone(), cx)))
}

fn control(id: &'static str, ic: IconName, p: Palette) -> Stateful<Div> {
    div()
        .id(id)
        .size(px(36.))
        .flex()
        .items_center()
        .justify_center()
        .rounded_full()
        .cursor_pointer()
        .hover(|s| s.bg(hsla(0., 0., 1., 0.12)))
        .child(icon(ic, 19., p.fg))
}

fn pill_button(
    id: &'static str,
    label: impl Into<SharedString>,
    ic: Option<IconName>,
    on: bool,
    p: Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .h(px(30.))
        .px(px(12.))
        .flex()
        .items_center()
        .gap(px(6.))
        .rounded_full()
        .cursor_pointer()
        .text_xs()
        .font_weight(FontWeight::MEDIUM)
        .border_1()
        .when(on, |d| {
            d.bg(p.accent_soft).border_color(p.accent).text_color(p.fg)
        })
        .when(!on, |d| {
            d.border_color(hsla(0., 0., 1., 0.18))
                .text_color(p.dim)
                .hover(|s| s.text_color(p.fg))
        })
        .when_some(ic, |d, ic| {
            d.child(icon(ic, 13., if on { p.accent } else { p.dim }))
        })
        .child(label.into())
}

#[cfg(test)]
mod tests {
    use super::{INFO_W, RAIL_W, room};
    use gpui_kit::{px, size};

    #[test]
    fn portrait_video_gets_the_side_room_it_leaves() {
        let wide = size(px(2560.), px(1440.));
        let r = room((720, 1280), wide);
        assert!(r.rail && r.info);
        let narrow = size(px(810. + RAIL_W + 10.), px(1440.));
        let r = room((720, 1280), narrow);
        assert!(r.rail && !r.info);
        let r = room(
            (720, 1280),
            size(px(810. + RAIL_W + INFO_W - 1.), px(1440.)),
        );
        assert!(!r.info);
        assert_eq!(room((1920, 1080), wide), super::Room::default());
        assert_eq!(room((0, 0), wide), super::Room::default());
    }
}
