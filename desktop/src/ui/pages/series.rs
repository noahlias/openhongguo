//! One series: a backdrop hero, the play/resume buttons, seasons, the numbered
//! episode grid with the TUI's states, and related titles
//! (`internal/tui/page_series.go`).

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use crate::sdk::SdkError;
use crate::sdk::types::{
    Album, Episode, EpisodeOptions, Page, Recommend, ResumePoint, Series, SeriesSummary,
};
use crate::ui::format;
use crate::ui::i18n::fill;
use crate::ui::icons::icon;
use crate::ui::layout::{self, Layout};
use crate::ui::state::{self, Nav, PlayRequest};
use crate::ui::theme::{Palette, palette};
use crate::ui::widgets::{self, Card};

pub struct SeriesPage {
    id: String,
    /// What the opener knew, shown until the details arrive.
    summary: SeriesSummary,
    series: Option<Series>,
    episodes: Vec<Episode>,
    album: Option<Album>,
    related: Option<Recommend>,
    error: Option<SdkError>,
    loading: bool,
    task: Option<Task<()>>,
    resume_task: Option<Task<()>>,
    scroll: ScrollHandle,
}

impl EventEmitter<Nav> for SeriesPage {}

impl SeriesPage {
    pub fn new(summary: SeriesSummary, cx: &mut Context<Self>) -> SeriesPage {
        cx.observe(&state::covers(cx), |_, _, cx| cx.notify())
            .detach();
        let mut page = SeriesPage {
            id: summary.id.clone(),
            summary,
            series: None,
            episodes: Vec::new(),
            album: None,
            related: None,
            error: None,
            loading: false,
            task: None,
            resume_task: None,
            scroll: ScrollHandle::new(),
        };
        page.load(cx);
        page
    }

    pub fn id(&self) -> &str {
        &self.id
    }

    fn load(&mut self, cx: &mut Context<Self>) {
        self.loading = true;
        self.error = None;
        let sdk = state::sdk(cx);
        let id = self.id.clone();
        let series = sdk.call::<Series>("getSeries", (id.clone(),));
        let album = sdk.call::<Album>("seasons", (id.clone(),));
        let related = sdk.call::<Recommend>("seriesRecommend", (id.clone(),));
        self.task = Some(cx.spawn(async move |this, cx| {
            let episodes = async {
                let mut all: Vec<Episode> = Vec::new();
                let mut cursor = String::new();
                loop {
                    let opts = EpisodeOptions {
                        cursor: cursor.clone(),
                        page_size: 200,
                    };
                    let page = sdk
                        .call::<Page<Episode>>("listEpisodes", (id.clone(), opts))
                        .await?;
                    all.extend(page.items);
                    if !page.has_more || page.cursor.is_empty() || page.cursor == cursor {
                        return Ok::<_, SdkError>(all);
                    }
                    cursor = page.cursor;
                }
            };
            let (series, episodes, album, related) =
                futures::join!(series, episodes, album, related);
            let _ = this.update(cx, |p, cx| {
                p.loading = false;
                match (series, episodes) {
                    (Ok(s), Ok(mut eps)) => {
                        eps.sort_by_key(|e| e.number);
                        p.summary = s.summary();
                        p.series = Some(s);
                        p.episodes = eps;
                    }
                    (Err(err), _) | (_, Err(err)) => p.error = Some(err),
                }
                // Seasons and related titles are extras; the page works without them.
                p.album = album.ok();
                p.related = related.ok();
                cx.notify();
            });
        }));
        cx.notify();
    }

    fn switch_season(&mut self, series_id: String, cx: &mut Context<Self>) {
        if series_id != self.id {
            self.id = series_id.clone();
            self.summary = SeriesSummary {
                id: series_id,
                ..Default::default()
            };
            self.series = None;
            self.episodes.clear();
            self.scroll.set_offset(point(px(0.), px(0.)));
            self.load(cx);
        }
    }

    fn refuse_book(&self, cx: &mut Context<Self>) -> bool {
        if self.series.as_ref().is_some_and(Series::is_book) {
            cx.emit(Nav::error(state::tr(cx).toast_book));
            return true;
        }
        false
    }

    fn play(&mut self, ep: Episode, start_seconds: f64, cx: &mut Context<Self>) {
        if self.refuse_book(cx) {
            return;
        }
        if !ep.playable {
            cx.emit(Nav::error(state::tr(cx).toast_locked));
            return;
        }
        cx.emit(Nav::Play(PlayRequest {
            series: self.summary.clone(),
            episodes: self.episodes.clone(),
            start: ep,
            start_seconds,
        }));
    }

    /// Continues from history (`resumePoint`: position − 3 s; all watched → ep 1).
    fn resume(&mut self, cx: &mut Context<Self>) {
        if self.episodes.is_empty() || self.refuse_book(cx) {
            return;
        }
        let call = state::sdk(cx)
            .call::<ResumePoint>("resumePoint", (self.id.clone(), self.episodes.len()));
        self.resume_task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |p, cx| match result {
                Ok(rp) => {
                    let ep = p
                        .episodes
                        .iter()
                        .find(|e| e.number == rp.episode)
                        .or(p.episodes.first())
                        .cloned();
                    if let Some(ep) = ep {
                        p.play(ep, rp.start as f64, cx);
                    }
                }
                Err(err) => cx.emit(Nav::error(err.describe(state::zh(cx)))),
            });
        }));
    }

    fn hero(&self, layout: Layout, cx: &mut Context<Self>) -> Div {
        let p = palette(cx);
        let t = state::tr(cx);
        let zh = state::zh(cx);
        let cover = state::cover(&self.summary.cover_url, cx);
        let watched = state::ohg(cx).history.get(&self.id).cloned();
        let s = self.series.as_ref();
        let mut pills: Vec<String> = Vec::new();
        if let Some(s) = s {
            pills.extend(
                [
                    format::status(s.status, zh),
                    format::content(s.content_type, zh),
                    format::pay(s.pay_type, zh),
                ]
                .map(str::to_string),
            );
        }
        if self.summary.episode_count > 0 {
            pills.push(fill(t.episodes, self.summary.episode_count));
        }
        pills.retain(|x| !x.is_empty());
        let stats: Vec<String> = s
            .map(|s| {
                [
                    (t.heat, s.heat),
                    (t.plays, s.play_count),
                    (t.follows, s.follow_count),
                    (t.likes, s.digg_count),
                ]
                .into_iter()
                .filter(|(_, n)| *n > 0)
                .map(|(l, n)| format!("{l} {}", format::count(n, zh)))
                .collect()
            })
            .unwrap_or_default();
        let resume_label = match &watched {
            Some(h) if h.episode > 0 => fill(t.resume, h.episode),
            _ => t.play.to_string(),
        };
        let first = self.episodes.first().cloned();
        let buttons = div()
            .flex()
            .gap(px(10.))
            .child(
                widgets::primary_button("series-play", resume_label, Some(IconName::Play), p)
                    .on_click(cx.listener(|this, _, _, cx| this.resume(cx))),
            )
            .when(watched.as_ref().is_some_and(|h| h.episode > 1), |d| {
                d.child(
                    widgets::ghost_button("series-first", t.play_from, None, p).on_click(
                        cx.listener(move |this, _, _, cx| {
                            if let Some(ep) = first.clone() {
                                this.play(ep, 0.0, cx);
                            }
                        }),
                    ),
                )
            });
        let compact = layout == Layout::Compact;
        let poster_w = if compact { 120. } else { 200. };
        let poster = div()
            .w(px(poster_w))
            .h(px(poster_w * 1.5))
            .flex_none()
            .rounded(px(14.))
            .overflow_hidden()
            .bg(p.raised)
            .border_1()
            .border_color(p.line)
            .shadow_lg()
            .when_some(cover.clone(), |d, c| {
                d.child(
                    img(ImageSource::Render(c))
                        .size_full()
                        .object_fit(ObjectFit::Cover),
                )
            });
        let backdrop = div()
            .absolute()
            .inset_0()
            .overflow_hidden()
            .when_some(cover, |d, c| {
                d.child(
                    img(ImageSource::Render(c))
                        .size_full()
                        .object_fit(ObjectFit::Cover)
                        .opacity(if p.dark { 0.22 } else { 0.16 }),
                )
            });
        let fade = div().absolute().inset_0().bg(linear_gradient(
            180.,
            linear_color_stop(p.bg.opacity(0.35), 0.),
            linear_color_stop(p.bg, 1.),
        ));
        div().relative().w_full().child(backdrop).child(fade).child(
            div()
                .relative()
                .px(px(layout.pad()))
                .pt(px(20.))
                .pb(px(28.))
                .flex()
                .flex_col()
                .gap(px(18.))
                .child(
                    back_button(t.back, p).on_click(cx.listener(|_, _, _, cx| cx.emit(Nav::Back))),
                )
                .child(
                    div()
                        .flex()
                        .gap(px(if compact { 16. } else { 28. }))
                        .child(poster)
                        .child(
                            div()
                                .flex_1()
                                .min_w_0()
                                .flex()
                                .flex_col()
                                .justify_end()
                                .gap(px(14.))
                                .child(
                                    div()
                                        .when(compact, |d| d.text_xl())
                                        .when(!compact, |d| d.text_3xl())
                                        .font_weight(FontWeight::BOLD)
                                        .text_color(p.fg)
                                        .child(if self.summary.title.is_empty() {
                                            t.loading.to_string()
                                        } else {
                                            self.summary.title.clone()
                                        }),
                                )
                                .child(
                                    div()
                                        .flex()
                                        .flex_wrap()
                                        .gap(px(6.))
                                        .children(pills.into_iter().map(|x| widgets::pill(x, p))),
                                )
                                .when(!stats.is_empty(), |d| {
                                    d.child(
                                        div()
                                            .text_sm()
                                            .text_color(p.dim)
                                            .child(stats.join("  ·  ")),
                                    )
                                })
                                .when(!self.summary.description.is_empty(), |d| {
                                    d.child(
                                        div()
                                            .max_w(px(760.))
                                            .text_sm()
                                            .line_height(rems(1.5))
                                            .text_color(p.dim)
                                            .line_clamp(if compact { 3 } else { 4 })
                                            .child(self.summary.description.clone()),
                                    )
                                })
                                .child(buttons),
                        ),
                ),
        )
    }

    fn seasons(&self, cx: &mut Context<Self>) -> Option<Div> {
        let album = self.album.as_ref().filter(|a| a.seasons.len() >= 2)?;
        let p = palette(cx);
        let t = state::tr(cx);
        Some(
            div()
                .flex()
                .flex_col()
                .gap(px(10.))
                .child(widgets::subtitle(t.seasons, p))
                .child(div().flex().flex_wrap().gap(px(8.)).children(
                    album.seasons.iter().enumerate().map(|(i, season)| {
                        let id = season.series_id.clone();
                        let label = if season.title.is_empty() {
                            format!("{}", season.index)
                        } else {
                            season.title.clone()
                        };
                        widgets::chip(("season", i), label, season.series_id == self.id, p)
                            .on_click(
                                cx.listener(move |this, _, _, cx| {
                                    this.switch_season(id.clone(), cx)
                                }),
                            )
                    }),
                )),
        )
    }

    fn episodes(&self, cx: &mut Context<Self>) -> Div {
        let p = palette(cx);
        let t = state::tr(cx);
        let watched = state::ohg(cx).history.get(&self.id).cloned();
        let current = watched.as_ref().map(|h| h.episode).unwrap_or(0);
        let finished = watched.as_ref().is_some_and(|h| h.finished);
        let tiles = self.episodes.iter().map(|ep| {
            let state = if !ep.playable {
                Tile::Locked
            } else if ep.number == current && !finished {
                Tile::Current
            } else if ep.number < current || (ep.number == current && finished) {
                Tile::Watched
            } else {
                Tile::Plain
            };
            let ep2 = ep.clone();
            tile(ep.number, state, ep.is_new, p)
                .on_click(cx.listener(move |this, _, _, cx| this.play(ep2.clone(), 0.0, cx)))
        });
        div()
            .flex()
            .flex_col()
            .gap(px(12.))
            .child(
                div()
                    .flex()
                    .items_center()
                    .gap(px(16.))
                    .child(widgets::subtitle(t.episode_list, p))
                    .child(legend(
                        &[
                            (t.legend_current, p.accent),
                            (t.legend_watched, p.faint),
                            (t.legend_new, p.green),
                        ],
                        p,
                    ))
                    .child(
                        div()
                            .flex()
                            .items_center()
                            .gap(px(4.))
                            .child(icon(IconName::Lock, 12., p.faint))
                            .child(div().text_xs().text_color(p.dim).child(t.legend_locked)),
                    ),
            )
            .child(div().flex().flex_wrap().gap(px(8.)).children(tiles))
    }

    fn related(&self, cols: u16, cx: &mut Context<Self>) -> Option<Div> {
        let rec = self.related.as_ref().filter(|r| !r.items.is_empty())?;
        let p = palette(cx);
        let t = state::tr(cx);
        let title = if rec.cell_title.is_empty() {
            t.related.to_string()
        } else {
            rec.cell_title.clone()
        };
        let mut cards = Vec::new();
        for (i, item) in rec.items.iter().take(usize::from(cols) * 2).enumerate() {
            let cover = state::cover(&item.cover_url, cx);
            let open = item.clone();
            let meta = if item.episode_count > 0 {
                fill(t.episodes, item.episode_count)
            } else {
                String::new()
            };
            cards.push(
                Card::new(
                    ("related", i),
                    item.title.clone(),
                    cover,
                    cx.listener(move |_, _, _, cx| cx.emit(Nav::Open(open.clone()))),
                )
                .meta(meta)
                .order(cards.len()),
            );
        }
        Some(
            div()
                .flex()
                .flex_col()
                .gap(px(14.))
                .child(widgets::subtitle(title, p))
                .child(widgets::grid(cols).children(cards)),
        )
    }
}

impl Render for SeriesPage {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let layout = Layout::current(window, cx);
        let cols = layout::columns(window, cx, widgets::CARD_MIN);
        let p = palette(cx);
        let t = state::tr(cx);
        let zh = state::zh(cx);
        let hero = self.hero(layout, cx);
        let body = if let Some(err) = &self.error {
            widgets::error_state(
                err.describe(zh),
                widgets::ghost_button("series-retry", t.retry, None, p)
                    .on_click(cx.listener(|this, _, _, cx| this.load(cx))),
                p,
            )
        } else if self.loading && self.series.is_none() {
            widgets::centered(t.loading, p)
        } else {
            div()
                .flex()
                .flex_col()
                .gap(px(30.))
                .children(self.seasons(cx))
                .child(self.episodes(cx))
                .children(self.related(cols, cx))
        };
        div()
            .id("series")
            .size_full()
            .overflow_y_scroll()
            .track_scroll(&self.scroll)
            .child(hero)
            .child(div().px(px(layout.pad())).pb(px(40.)).child(body))
    }
}

#[derive(Clone, Copy, PartialEq)]
enum Tile {
    Plain,
    Current,
    Watched,
    Locked,
}

fn tile(n: i32, state: Tile, is_new: bool, p: Palette) -> Stateful<Div> {
    div()
        .id(("ep", n as usize))
        .relative()
        .w(px(58.))
        .h(px(40.))
        .flex()
        .items_center()
        .justify_center()
        .rounded(px(9.))
        .border_1()
        .cursor_pointer()
        .text_sm()
        .map(|d| match state {
            Tile::Current => d
                .border_color(p.accent)
                .bg(p.accent_soft)
                .text_color(p.accent)
                .font_weight(FontWeight::SEMIBOLD),
            Tile::Watched => d
                .border_color(p.line)
                .bg(p.sidebar)
                .text_color(p.faint)
                .hover(|s| s.border_color(p.dim)),
            Tile::Locked => d.border_color(p.line).text_color(p.faint),
            Tile::Plain => d
                .border_color(p.line)
                .bg(p.raised)
                .text_color(p.fg)
                .hover(|s| s.border_color(p.accent).text_color(p.accent)),
        })
        .child(n.to_string())
        .when(state == Tile::Locked, |d| {
            d.child(div().absolute().top(px(3.)).right(px(4.)).child(icon(
                IconName::Lock,
                10.,
                p.faint,
            )))
        })
        .when(is_new && state != Tile::Locked, |d| {
            d.child(
                div()
                    .absolute()
                    .top(px(4.))
                    .right(px(5.))
                    .size(px(6.))
                    .rounded_full()
                    .bg(p.green),
            )
        })
}

fn legend(items: &[(&'static str, Hsla)], p: Palette) -> Div {
    div()
        .flex()
        .gap(px(12.))
        .children(items.iter().map(|(label, c)| {
            div()
                .flex()
                .items_center()
                .gap(px(5.))
                .child(div().size(px(8.)).rounded_full().bg(*c))
                .child(div().text_xs().text_color(p.dim).child(*label))
        }))
}

fn back_button(label: &'static str, p: Palette) -> Stateful<Div> {
    div()
        .id("series-back")
        .flex()
        .items_center()
        .gap(px(4.))
        .w_auto()
        .self_start()
        .px(px(10.))
        .h(px(30.))
        .rounded_full()
        .cursor_pointer()
        .text_sm()
        .text_color(p.dim)
        .hover(|s| s.bg(p.hover).text_color(p.fg))
        .child(icon(IconName::ChevronLeft, 16., p.dim))
        .child(label)
}
