//! Discover: channel tabs, a spotlight on the first picks, and a cover grid
//! that pages by cursor until the window is full, then on scroll.

use std::time::Duration;

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use crate::sdk::SdkError;
use crate::sdk::types::{DiscoverOptions, Page, SeriesSummary};
use crate::ui::i18n::fill;
use crate::ui::layout::{self, Layout};
use crate::ui::motion;
use crate::ui::state::{self, Nav};
use crate::ui::theme::palette;
use crate::ui::widgets::{self, Card};

/// The spotlight's picks; the grid starts after them.
const PICKS: usize = 6;
const HERO_H: f32 = 340.0;
const HERO_H_COMPACT: f32 = 230.0;
const ROTATE_EVERY: Duration = Duration::from_secs(7);

/// The service's channel names (`internal/tui/labels.go`). 漫画/小说/听书 hold
/// books, which the desktop app cannot open yet, so they are left out.
const CHANNELS: [(&str, &str); 9] = [
    ("推荐", "Featured"),
    ("真人剧", "Live action"),
    ("漫剧", "Motion comics"),
    ("电影", "Movies"),
    ("找剧", "Browse"),
    ("经典", "Classics"),
    ("知识", "Knowledge"),
    ("大视听", "Big screen"),
    ("社区", "Community"),
];

pub struct DiscoverPage {
    channel: usize,
    items: Vec<SeriesSummary>,
    cursor: String,
    has_more: bool,
    loading: bool,
    error: Option<SdkError>,
    task: Option<Task<()>>,
    scroll: ScrollHandle,
    spot: usize,
    rotate: Task<()>,
    /// `items.len()` when the page last checked whether it fills the window.
    filled_at: usize,
}

impl EventEmitter<Nav> for DiscoverPage {}

impl DiscoverPage {
    pub fn new(cx: &mut Context<Self>) -> DiscoverPage {
        cx.observe(&state::covers(cx), |_, _, cx| cx.notify())
            .detach();
        let mut page = DiscoverPage {
            channel: 0,
            items: Vec::new(),
            cursor: String::new(),
            has_more: false,
            loading: false,
            error: None,
            task: None,
            scroll: ScrollHandle::new(),
            spot: 0,
            rotate: Self::rotate(cx),
            filled_at: 0,
        };
        page.load(true, cx);
        page
    }

    /// Steps the spotlight to the next pick every few seconds. Picking one
    /// by hand restarts the wait.
    fn rotate(cx: &mut Context<Self>) -> Task<()> {
        cx.spawn(async move |this, cx| {
            loop {
                cx.background_executor().timer(ROTATE_EVERY).await;
                let alive = this.update(cx, |p, cx| {
                    if p.items.len() >= PICKS {
                        p.spot = (p.spot + 1) % PICKS;
                        cx.notify();
                    }
                });
                if alive.is_err() {
                    return;
                }
            }
        })
    }

    fn select(&mut self, channel: usize, cx: &mut Context<Self>) {
        if channel != self.channel || self.error.is_some() {
            self.channel = channel;
            self.scroll.set_offset(point(px(0.), px(0.)));
            self.load(true, cx);
        }
    }

    fn load(&mut self, reset: bool, cx: &mut Context<Self>) {
        if reset {
            self.items.clear();
            self.cursor.clear();
            self.has_more = false;
            self.spot = 0;
            self.filled_at = 0;
        }
        self.loading = true;
        self.error = None;
        let opts = DiscoverOptions {
            cursor: self.cursor.clone(),
            channel: CHANNELS[self.channel].0.to_string(),
        };
        let call = state::sdk(cx).call::<Page<SeriesSummary>>("discover", (opts,));
        self.task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |p, cx| {
                p.loading = false;
                match result {
                    Ok(page) => {
                        let seen: std::collections::HashSet<String> =
                            p.items.iter().map(|i| i.id.clone()).collect();
                        p.items
                            .extend(page.items.into_iter().filter(|i| !seen.contains(&i.id)));
                        p.cursor = page.cursor;
                        p.has_more = page.has_more && !p.cursor.is_empty();
                    }
                    Err(err) => p.error = Some(err),
                }
                cx.notify();
            });
        }));
        cx.notify();
    }

    fn maybe_more(&mut self, cx: &mut Context<Self>) {
        let (offset, max) = (self.scroll.offset().y, self.scroll.max_offset().y);
        if self.has_more && !self.loading && -offset >= max - px(700.) {
            self.load(false, cx);
        }
    }
}

impl DiscoverPage {
    fn hero(&self, wide: bool, compact: bool, cx: &mut Context<Self>) -> Div {
        let p = palette(cx);
        let t = state::tr(cx);
        let item = self.items[self.spot].clone();
        let cover = state::cover(&item.cover_url, cx);
        let mut meta = Vec::new();
        if item.episode_count > 0 {
            meta.push(fill(t.episodes, item.episode_count));
        }
        meta.extend(
            [item.category.clone(), item.author.clone()]
                .into_iter()
                .filter(|s| !s.is_empty()),
        );
        let open = item.clone();
        let picks: Vec<_> = self.items[..PICKS]
            .iter()
            .enumerate()
            .map(|(i, pick)| {
                let on = i == self.spot;
                div()
                    .id(("spot", i))
                    .w(px(84.))
                    .aspect_ratio(2. / 3.)
                    .rounded(px(10.))
                    .overflow_hidden()
                    .cursor_pointer()
                    .bg(p.raised)
                    .border_2()
                    .border_color(if on { p.accent } else { hsla(0., 0., 0., 0.) })
                    .when(!on, |d| d.opacity(0.7).hover(|s| s.opacity(1.)))
                    .when_some(state::cover(&pick.cover_url, cx), |d, c| {
                        d.child(
                            img(ImageSource::Render(c))
                                .size_full()
                                .object_fit(ObjectFit::Cover),
                        )
                    })
                    .on_click(cx.listener(move |this, _, _, cx| {
                        this.spot = i;
                        this.rotate = Self::rotate(cx);
                        cx.notify();
                    }))
            })
            .collect();
        div()
            .relative()
            .w_full()
            .h(px(if compact { HERO_H_COMPACT } else { HERO_H }))
            .rounded(px(20.))
            .overflow_hidden()
            .bg(p.raised)
            .border_1()
            .border_color(p.line)
            .when_some(cover.clone(), |d, c| {
                d.child(motion::enter(
                    div().absolute().inset_0().child(
                        img(ImageSource::Render(c))
                            .size_full()
                            .object_fit(ObjectFit::Cover)
                            .opacity(if p.dark { 0.32 } else { 0.22 }),
                    ),
                    ("spot-bg", self.spot),
                    700,
                    Duration::ZERO,
                    0.,
                ))
            })
            .child(div().absolute().inset_0().bg(linear_gradient(
                90.,
                linear_color_stop(p.bg.opacity(0.95), 0.),
                linear_color_stop(p.bg.opacity(0.25), 1.),
            )))
            .child(
                div()
                    .relative()
                    .size_full()
                    .p(px(if compact { 16. } else { 28. }))
                    .flex()
                    .items_center()
                    .gap(px(if compact { 16. } else { 28. }))
                    .child(motion::enter(
                        div()
                            .id("spot-poster")
                            .h_full()
                            .aspect_ratio(2. / 3.)
                            .flex_none()
                            .rounded(px(14.))
                            .overflow_hidden()
                            .bg(p.raised)
                            .shadow_lg()
                            .cursor_pointer()
                            .when_some(cover, |d, c| {
                                d.child(
                                    img(ImageSource::Render(c))
                                        .size_full()
                                        .object_fit(ObjectFit::Cover),
                                )
                            })
                            .on_click({
                                let open = open.clone();
                                cx.listener(move |_, _, _, cx| cx.emit(Nav::Open(open.clone())))
                            }),
                        ("spot-poster-in", self.spot),
                        420,
                        Duration::ZERO,
                        0.,
                    ))
                    .child(motion::enter(
                        div()
                            .flex_1()
                            .min_w_0()
                            .flex()
                            .flex_col()
                            .gap(px(12.))
                            .child(
                                div().flex().child(
                                    div()
                                        .px(px(10.))
                                        .py(px(3.))
                                        .rounded_full()
                                        .bg(p.accent_soft)
                                        .text_xs()
                                        .font_weight(FontWeight::SEMIBOLD)
                                        .text_color(p.accent)
                                        .child(t.spotlight),
                                ),
                            )
                            .child(
                                div()
                                    .when(compact, |d| d.text_xl())
                                    .when(!compact, |d| d.text_3xl())
                                    .font_weight(FontWeight::BOLD)
                                    .text_color(p.fg)
                                    .line_clamp(2)
                                    .child(item.title.clone()),
                            )
                            .when(!meta.is_empty(), |d| {
                                d.child(div().text_sm().text_color(p.dim).child(meta.join(" · ")))
                            })
                            .when(!compact && !item.description.is_empty(), |d| {
                                d.child(
                                    div()
                                        .max_w(px(680.))
                                        .text_sm()
                                        .text_color(p.dim)
                                        .line_clamp(3)
                                        .child(item.description.clone()),
                                )
                            })
                            .child(
                                div().pt(px(6.)).flex().child(
                                    widgets::primary_button(
                                        "spot-open",
                                        t.details,
                                        Some(IconName::Play),
                                        p,
                                    )
                                    .on_click(cx.listener(
                                        move |_, _, _, cx| cx.emit(Nav::Open(open.clone())),
                                    )),
                                ),
                            ),
                        ("spot-text", self.spot),
                        520,
                        Duration::from_millis(60),
                        10.,
                    ))
                    .when(wide, |d| {
                        d.child(
                            div()
                                .flex_none()
                                .grid()
                                .grid_cols(3)
                                .gap(px(10.))
                                .children(picks),
                        )
                    }),
            )
    }
}

impl Render for DiscoverPage {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let p = palette(cx);
        let t = state::tr(cx);
        let zh = state::zh(cx);
        let tabs = div()
            .flex()
            .flex_wrap()
            .gap(px(8.))
            .children(CHANNELS.iter().enumerate().map(|(i, (z, e))| {
                widgets::chip(
                    ("channel", i),
                    if zh { *z } else { *e },
                    i == self.channel,
                    p,
                )
                .on_click(cx.listener(move |this, _, _, cx| this.select(i, cx)))
            }));
        // Short first pages leave the window half empty; keep paging until it scrolls.
        if self.has_more && !self.loading && self.filled_at != self.items.len() {
            self.filled_at = self.items.len();
            cx.on_next_frame(window, |this, _, cx| this.maybe_more(cx));
        }
        let layout = Layout::current(window, cx);
        let cols = layout::columns(window, cx, widgets::CARD_MIN);
        let spotlight = self.items.len() >= PICKS + usize::from(cols);
        let wide = layout::usable_width(window, cx) >= 1180.;
        let skip = if spotlight { PICKS } else { 0 };
        let mut cards = Vec::with_capacity(self.items.len());
        for (i, item) in self.items.iter().enumerate().skip(skip) {
            let cover = state::cover(&item.cover_url, cx);
            let open = item.clone();
            let meta = if item.episode_count > 0 {
                fill(t.episodes, item.episode_count)
            } else {
                item.category.clone()
            };
            cards.push(
                Card::new(
                    ("discover", i),
                    item.title.clone(),
                    cover,
                    cx.listener(move |_, _, _, cx| cx.emit(Nav::Open(open.clone()))),
                )
                .meta(meta)
                .order(cards.len()),
            );
        }
        let footer = if let Some(err) = &self.error {
            widgets::error_state(
                err.describe(zh),
                widgets::ghost_button("discover-retry", t.retry, None, p)
                    .on_click(cx.listener(|this, _, _, cx| this.load(this.items.is_empty(), cx))),
                p,
            )
            .into_any_element()
        } else if self.loading {
            widgets::centered(t.loading, p).into_any_element()
        } else if self.items.is_empty() {
            widgets::centered(t.empty, p).into_any_element()
        } else if self.has_more {
            div()
                .w_full()
                .py(px(28.))
                .flex()
                .justify_center()
                .child(
                    widgets::ghost_button("discover-more", t.load_more, None, p)
                        .on_click(cx.listener(|this, _, _, cx| this.load(false, cx))),
                )
                .into_any_element()
        } else {
            div().h(px(32.)).into_any_element()
        };
        div()
            .id("discover")
            .size_full()
            .overflow_y_scroll()
            .track_scroll(&self.scroll)
            .on_scroll(cx.listener(|this, _, _, cx| this.maybe_more(cx)))
            .child(
                div()
                    .px(px(layout.pad()))
                    .pt(px(28.))
                    .pb(px(24.))
                    .flex()
                    .flex_col()
                    .gap(px(22.))
                    .child(widgets::title(t.discover, p))
                    .child(tabs)
                    .when(spotlight, |d| {
                        d.child(self.hero(wide, layout == Layout::Compact, cx))
                    })
                    .child(widgets::grid(cols).children(cards))
                    .child(footer),
            )
    }
}
