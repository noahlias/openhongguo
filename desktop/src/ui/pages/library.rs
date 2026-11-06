//! Continue (unfinished series as covers; a click resumes, then picks related
//! to the latest one) and History (every entry as a tile; clear with a confirm). Both read `history.json`, shared
//! with the CLI and TUI.

use std::collections::HashMap;

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use crate::sdk::types::{HistoryEntry, Recommend, ResumePoint, SeriesDetail};
use crate::ui::format;
use crate::ui::i18n::fill;
use crate::ui::icons::icon;
use crate::ui::layout::{self, Layout};
use crate::ui::state::{self, Nav, Ohg, PlayRequest};
use crate::ui::theme::{Palette, palette};
use crate::ui::widgets::{self, Card};

/// History tiles grow from this width until another column fits.
const HISTORY_MIN: f32 = 340.0;

#[derive(Clone, Copy, PartialEq)]
pub enum Mode {
    Continue,
    History,
}

pub struct LibraryPage {
    mode: Mode,
    details: HashMap<String, SeriesDetail>,
    requested: std::collections::HashSet<String>,
    confirm_clear: bool,
    tasks: Vec<Task<()>>,
    scroll: ScrollHandle,
    /// Related to the latest entry: (its series ID, its title, the picks).
    picks: Option<(String, String, Recommend)>,
    picks_task: Option<Task<()>>,
}

impl EventEmitter<Nav> for LibraryPage {}

impl LibraryPage {
    pub fn new(mode: Mode, cx: &mut Context<Self>) -> LibraryPage {
        cx.observe(&state::covers(cx), |_, _, cx| cx.notify())
            .detach();
        cx.observe_global::<Ohg>(|this: &mut LibraryPage, cx| {
            this.fetch_details(cx);
            cx.notify();
        })
        .detach();
        let mut page = LibraryPage {
            mode,
            details: HashMap::new(),
            requested: Default::default(),
            confirm_clear: false,
            tasks: Vec::new(),
            scroll: ScrollHandle::new(),
            picks: None,
            picks_task: None,
        };
        page.fetch_details(cx);
        page
    }

    fn entries(&self, cx: &App) -> Vec<HistoryEntry> {
        let mut list: Vec<HistoryEntry> = state::ohg(cx).history.values().cloned().collect();
        if self.mode == Mode::Continue {
            list.retain(|e| !e.completed());
        }
        list.sort_by(|a, b| b.updated_at.cmp(&a.updated_at));
        list
    }

    fn fetch_picks(&mut self, cx: &mut Context<Self>) {
        let Some(latest) = self.entries(cx).into_iter().next() else {
            return;
        };
        if self
            .picks
            .as_ref()
            .is_some_and(|(id, _, _)| *id == latest.series_id)
        {
            return;
        }
        let call = state::sdk(cx).call::<Recommend>("seriesRecommend", (latest.series_id.clone(),));
        self.picks_task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |p, cx| {
                match result {
                    Ok(rec) => p.picks = Some((latest.series_id, latest.title, rec)),
                    Err(err) => log::warn!("continue picks: {err}"),
                }
                cx.notify();
            });
        }));
    }

    fn render_picks(&self, cols: u16, cx: &mut Context<Self>) -> Option<Div> {
        let (_, title, rec) = self.picks.as_ref()?;
        let p = palette(cx);
        let t = state::tr(cx);
        let watched = &state::ohg(cx).history;
        let items: Vec<_> = rec
            .items
            .iter()
            .filter(|i| !watched.contains_key(&i.id))
            .take(usize::from(cols) * 3)
            .cloned()
            .collect();
        if items.is_empty() {
            return None;
        }
        let mut cards = Vec::new();
        for (i, item) in items.into_iter().enumerate() {
            let cover = state::cover(&item.cover_url, cx);
            let meta = if item.episode_count > 0 {
                fill(t.episodes, item.episode_count)
            } else {
                item.category.clone()
            };
            cards.push(
                Card::new(
                    ("picks", i),
                    item.title.clone(),
                    cover,
                    cx.listener(move |_, _, _, cx| cx.emit(Nav::Open(item.clone()))),
                )
                .meta(meta)
                .order(cards.len()),
            );
        }
        Some(
            div()
                .pt(px(10.))
                .flex()
                .flex_col()
                .gap(px(16.))
                .child(widgets::subtitle(fill(t.because_watched, title), p))
                .child(widgets::grid(cols).children(cards)),
        )
    }

    /// Covers and episode lists for entries not seen yet, in batches.
    fn fetch_details(&mut self, cx: &mut Context<Self>) {
        if self.mode == Mode::Continue {
            self.fetch_picks(cx);
        }
        let missing: Vec<String> = self
            .entries(cx)
            .into_iter()
            .map(|e| e.series_id)
            .filter(|id| !self.requested.contains(id))
            .take(60)
            .collect();
        if missing.is_empty() {
            return;
        }
        self.requested.extend(missing.iter().cloned());
        for chunk in missing.chunks(20) {
            let call =
                state::sdk(cx).call::<Vec<SeriesDetail>>("getSeriesBatch", (chunk.to_vec(),));
            self.tasks.push(cx.spawn(async move |this, cx| {
                let result = call.await;
                let _ = this.update(cx, |p, cx| {
                    match result {
                        Ok(list) => {
                            for d in list {
                                if let Some(id) = d.series.as_ref().map(|s| s.id.clone()) {
                                    p.details.insert(id, d);
                                }
                            }
                        }
                        Err(err) => log::warn!("history covers: {err}"),
                    }
                    cx.notify();
                });
            }));
        }
    }

    fn resume(&mut self, entry: HistoryEntry, cx: &mut Context<Self>) {
        let Some(detail) = self.details.get(&entry.series_id).cloned() else {
            cx.emit(Nav::Open(crate::sdk::types::SeriesSummary {
                id: entry.series_id,
                title: entry.title,
                ..Default::default()
            }));
            return;
        };
        let Some(series) = detail.series else { return };
        if series.is_book() {
            cx.emit(Nav::error(state::tr(cx).toast_book));
            return;
        }
        let mut episodes = detail.episodes;
        episodes.sort_by_key(|e| e.number);
        let call =
            state::sdk(cx).call::<ResumePoint>("resumePoint", (series.id.clone(), episodes.len()));
        self.tasks.push(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |_, cx| match result {
                Ok(rp) => {
                    let Some(start) = episodes
                        .iter()
                        .find(|e| e.number == rp.episode)
                        .or(episodes.first())
                        .cloned()
                    else {
                        return;
                    };
                    if !start.playable {
                        cx.emit(Nav::Open(series.summary()));
                        return;
                    }
                    cx.emit(Nav::Play(PlayRequest {
                        series: series.summary(),
                        episodes,
                        start,
                        start_seconds: rp.start as f64,
                    }));
                }
                Err(err) => cx.emit(Nav::error(err.describe(state::zh(cx)))),
            });
        }));
    }

    fn clear(&mut self, cx: &mut Context<Self>) {
        self.confirm_clear = false;
        let call = state::sdk(cx).call::<serde_json::Value>("historyClear", ());
        self.tasks.push(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |_, cx| match result {
                Ok(_) => {
                    cx.emit(Nav::toast(state::tr(cx).cleared));
                    state::reload_history(cx).detach();
                }
                Err(err) => cx.emit(Nav::error(err.describe(state::zh(cx)))),
            });
        }));
        cx.notify();
    }

    fn render_continue(
        &mut self,
        entries: Vec<HistoryEntry>,
        cols: u16,
        cx: &mut Context<Self>,
    ) -> Div {
        let t = state::tr(cx);
        let mut cards = Vec::new();
        for (i, e) in entries.into_iter().enumerate() {
            let detail = self.details.get(&e.series_id);
            let cover_url = detail
                .and_then(|d| d.series.as_ref())
                .map(|s| s.cover_url.clone())
                .unwrap_or_default();
            let cover = state::cover(&cover_url, cx);
            let total = if e.episodes > 0 {
                e.episodes
            } else {
                detail
                    .and_then(|d| d.series.as_ref())
                    .map(|s| s.episode_count)
                    .unwrap_or(0)
            };
            let progress = (total > 0).then(|| e.episode as f32 / total as f32);
            let title = if e.title.is_empty() {
                e.series_id.clone()
            } else {
                e.title.clone()
            };
            let meta = format!("{} {}", t.last_watched, fill(t.episode_n, e.episode));
            let badge = if total > 0 {
                format!("{}/{}", e.episode, total)
            } else {
                String::new()
            };
            cards.push(
                Card::new(
                    ("continue", i),
                    title,
                    cover,
                    cx.listener(move |this, _, _, cx| this.resume(e.clone(), cx)),
                )
                .meta(meta)
                .badge(badge)
                .progress(progress)
                .order(cards.len()),
            );
        }
        widgets::grid(cols).children(cards)
    }

    fn render_history(
        &mut self,
        entries: Vec<HistoryEntry>,
        cols: u16,
        cx: &mut Context<Self>,
    ) -> Div {
        let p = palette(cx);
        let t = state::tr(cx);
        let mut rows = Vec::new();
        for (i, e) in entries.into_iter().enumerate() {
            let series = self
                .details
                .get(&e.series_id)
                .and_then(|d| d.series.clone());
            let cover = state::cover(
                series.as_ref().map(|s| s.cover_url.as_str()).unwrap_or(""),
                cx,
            );
            let summary =
                series
                    .map(|s| s.summary())
                    .unwrap_or_else(|| crate::sdk::types::SeriesSummary {
                        id: e.series_id.clone(),
                        title: e.title.clone(),
                        ..Default::default()
                    });
            rows.push(
                history_row(i, &e, cover, t, p)
                    .on_click(cx.listener(move |_, _, _, cx| cx.emit(Nav::Open(summary.clone())))),
            );
        }
        div().grid().grid_cols(cols).gap(px(14.)).children(rows)
    }
}

impl Render for LibraryPage {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let layout = Layout::current(window, cx);
        let p = palette(cx);
        let t = state::tr(cx);
        let entries = self.entries(cx);
        let empty = entries.is_empty();
        let header = div()
            .flex()
            .items_center()
            .justify_between()
            .child(widgets::title(
                if self.mode == Mode::Continue {
                    t.continue_
                } else {
                    t.history
                },
                p,
            ))
            .when(self.mode == Mode::History && !empty, |d| {
                if self.confirm_clear {
                    d.child(
                        div()
                            .flex()
                            .items_center()
                            .gap(px(10.))
                            .child(div().text_sm().text_color(p.dim).child(t.confirm_clear))
                            .child(
                                widgets::primary_button("clear-yes", t.yes, None, p)
                                    .on_click(cx.listener(|this, _, _, cx| this.clear(cx))),
                            )
                            .child(widgets::ghost_button("clear-no", t.no, None, p).on_click(
                                cx.listener(|this, _, _, cx| {
                                    this.confirm_clear = false;
                                    cx.notify();
                                }),
                            )),
                    )
                } else {
                    d.child(
                        widgets::ghost_button("clear", t.clear_history, Some(IconName::Delete), p)
                            .on_click(cx.listener(|this, _, _, cx| {
                                this.confirm_clear = true;
                                cx.notify();
                            })),
                    )
                }
            });
        let body = if empty {
            widgets::centered(
                if self.mode == Mode::Continue {
                    t.empty_continue
                } else {
                    t.empty_history
                },
                p,
            )
        } else if self.mode == Mode::Continue {
            let cols = layout::columns(window, cx, widgets::CARD_MIN);
            let picks = self.render_picks(cols, cx);
            div()
                .flex()
                .flex_col()
                .gap(px(22.))
                .child(self.render_continue(entries, cols, cx))
                .children(picks)
        } else {
            self.render_history(entries, layout::columns(window, cx, HISTORY_MIN), cx)
        };
        div()
            .id(if self.mode == Mode::Continue {
                "continue"
            } else {
                "history"
            })
            .size_full()
            .overflow_y_scroll()
            .track_scroll(&self.scroll)
            .child(
                div()
                    .px(px(layout.pad()))
                    .pt(px(28.))
                    .pb(px(40.))
                    .flex()
                    .flex_col()
                    .gap(px(22.))
                    .child(header)
                    .child(body),
            )
    }
}

fn history_row(
    i: usize,
    e: &HistoryEntry,
    cover: Option<std::sync::Arc<RenderImage>>,
    t: &crate::ui::i18n::Tr,
    p: Palette,
) -> Stateful<Div> {
    let title = if e.title.is_empty() {
        e.series_id.clone()
    } else {
        e.title.clone()
    };
    let mut detail = fill(t.episode_n, e.episode);
    if e.episodes > 0 {
        detail = format!("{detail} / {}", fill(t.episodes, e.episodes));
    }
    if e.position > 0.0 && !e.finished {
        detail = format!("{detail} · {}", format::clock(e.position));
    }
    div()
        .id(("history-row", i))
        .flex()
        .gap(px(14.))
        .p(px(10.))
        .rounded(px(14.))
        .bg(p.raised)
        .border_1()
        .border_color(p.line)
        .cursor_pointer()
        .hover(|s| s.border_color(p.accent))
        .child(
            div()
                .w(px(68.))
                .aspect_ratio(2. / 3.)
                .flex_none()
                .rounded(px(9.))
                .overflow_hidden()
                .bg(p.hover)
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
                .flex_1()
                .min_w_0()
                .py(px(2.))
                .flex()
                .flex_col()
                .gap(px(5.))
                .child(
                    div()
                        .text_sm()
                        .font_weight(FontWeight::MEDIUM)
                        .text_color(p.fg)
                        .line_clamp(2)
                        .child(title),
                )
                .child(div().text_xs().text_color(p.dim).child(detail))
                .child(div().flex_1())
                .child(
                    div()
                        .flex()
                        .items_center()
                        .justify_between()
                        .child(
                            div()
                                .text_xs()
                                .text_color(p.faint)
                                .child(format::date(&e.updated_at)),
                        )
                        .when(e.finished, |d| {
                            d.child(
                                div()
                                    .flex()
                                    .items_center()
                                    .gap(px(4.))
                                    .child(icon(IconName::Check, 13., p.green))
                                    .child(div().text_xs().text_color(p.green).child(t.finished)),
                            )
                        }),
                ),
        )
}
