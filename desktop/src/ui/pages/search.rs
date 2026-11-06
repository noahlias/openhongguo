//! Search: a debounced suggestion list under the field, result tabs, and the
//! 综合-only filters (`internal/tui/page_search.go`).

use std::time::Duration;

use gpui_kit::assets::IconName;
use gpui_kit::component::input::{Input, InputEvent, InputState};
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use crate::sdk::SdkError;
use crate::sdk::types::{Page, SearchOptions, SeriesDetail, SeriesSummary, Suggestion};
use crate::ui::i18n::fill;
use crate::ui::icons::icon;
use crate::ui::layout::{self, Layout};
use crate::ui::state::{self, Nav};
use crate::ui::theme::palette;
use crate::ui::widgets::{self, Card};

const TABS: [&str; 4] = ["综合", "剧集", "视频", "小说"];
const FILTERS: [(&str, &str, &str); 5] = [
    ("content_type_short_play", "真人剧", "Live action"),
    ("content_type_dynamic_comic", "漫剧", "Motion comic"),
    ("content_type_ai_dynamic_comic", "AI剧", "AI drama"),
    ("content_type_long_video", "影视剧", "Film & TV"),
    ("search_range_unread", "未看过", "Unwatched"),
];
const SUGGEST_DELAY: Duration = Duration::from_millis(250);

pub struct SearchPage {
    input: Entity<InputState>,
    query: String,
    tab: usize,
    filters: Vec<&'static str>,
    suggestions: Vec<Suggestion>,
    items: Vec<SeriesSummary>,
    cursor: String,
    has_more: bool,
    loading: bool,
    searched: bool,
    error: Option<SdkError>,
    search_task: Option<Task<()>>,
    suggest_task: Option<Task<()>>,
    cover_tasks: Vec<Task<()>>,
    scroll: ScrollHandle,
    /// `items.len()` when the page last checked whether it fills the window.
    filled_at: usize,
}

impl EventEmitter<Nav> for SearchPage {}

impl SearchPage {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> SearchPage {
        let placeholder = state::tr(cx).search_placeholder;
        let input = cx.new(|cx| {
            InputState::new(window, cx)
                .placeholder(placeholder)
                .clean_on_escape()
        });
        cx.subscribe_in(&input, window, |this, _, ev: &InputEvent, _, cx| match ev {
            InputEvent::Change => this.on_change(cx),
            InputEvent::PressEnter { .. } => {
                let q = this.input.read(cx).value().to_string();
                this.run(q, cx);
            }
            _ => {}
        })
        .detach();
        cx.observe(&state::covers(cx), |_, _, cx| cx.notify())
            .detach();
        let page = SearchPage {
            input,
            query: String::new(),
            tab: 0,
            filters: Vec::new(),
            suggestions: Vec::new(),
            items: Vec::new(),
            cursor: String::new(),
            has_more: false,
            loading: false,
            searched: false,
            error: None,
            search_task: None,
            suggest_task: None,
            cover_tasks: Vec::new(),
            scroll: ScrollHandle::new(),
            filled_at: 0,
        };
        page.load_cue(window, cx);
        page
    }

    pub fn focus(&self, window: &mut Window, cx: &mut App) {
        self.input.update(cx, |i, cx| i.focus(window, cx));
    }

    /// A rotating hint from the service replaces the static placeholder.
    fn load_cue(&self, window: &mut Window, cx: &mut Context<Self>) {
        let call = state::sdk(cx).call::<Vec<String>>("searchCues", ());
        cx.spawn_in(window, async move |this, cx| {
            if let Ok(cues) = call.await
                && let Some(cue) = cues.into_iter().find(|c| !c.trim().is_empty())
            {
                let _ = this.update_in(cx, |p, window, cx| {
                    p.input
                        .update(cx, |i, cx| i.set_placeholder(cue, window, cx))
                });
            }
        })
        .detach();
    }

    fn on_change(&mut self, cx: &mut Context<Self>) {
        let prefix = self.input.read(cx).value().trim().to_string();
        if prefix.is_empty() {
            self.suggestions.clear();
            self.suggest_task = None;
            cx.notify();
            return;
        }
        let sdk = state::sdk(cx);
        self.suggest_task = Some(cx.spawn(async move |this, cx| {
            cx.background_executor().timer(SUGGEST_DELAY).await;
            let result = sdk.call::<Vec<Suggestion>>("suggest", (prefix,)).await;
            let _ = this.update(cx, |p, cx| {
                p.suggestions = result.unwrap_or_default().into_iter().take(8).collect();
                cx.notify();
            });
        }));
    }

    fn pick(&mut self, s: Suggestion, window: &mut Window, cx: &mut Context<Self>) {
        let keyword = if s.keyword.is_empty() {
            s.text.clone()
        } else {
            s.keyword.clone()
        };
        // An all-digit keyword is a series ID: open it rather than search for it.
        if !keyword.is_empty() && keyword.chars().all(|c| c.is_ascii_digit()) {
            self.suggestions.clear();
            cx.emit(Nav::Open(SeriesSummary {
                id: keyword,
                title: s.text,
                ..Default::default()
            }));
            return;
        }
        self.input
            .update(cx, |i, cx| i.set_value(keyword.clone(), window, cx));
        self.run(keyword, cx);
    }

    fn run(&mut self, query: String, cx: &mut Context<Self>) {
        let query = query.trim().to_string();
        if query.is_empty() {
            return;
        }
        self.query = query;
        self.suggestions.clear();
        self.suggest_task = None;
        self.scroll.set_offset(point(px(0.), px(0.)));
        self.load(true, cx);
    }

    fn load(&mut self, reset: bool, cx: &mut Context<Self>) {
        if self.query.is_empty() {
            return;
        }
        if reset {
            self.cover_tasks.clear();
            self.items.clear();
            self.cursor.clear();
            self.has_more = false;
            self.filled_at = 0;
        }
        self.searched = true;
        self.loading = true;
        self.error = None;
        let opts = SearchOptions {
            cursor: self.cursor.clone(),
            page_size: 20,
            tab: TABS[self.tab].to_string(),
            filters: if self.tab == 0 {
                self.filters.iter().map(|f| f.to_string()).collect()
            } else {
                Vec::new()
            },
        };
        let call = state::sdk(cx).call::<Page<SeriesSummary>>("search", (self.query.clone(), opts));
        self.search_task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |p, cx| {
                p.loading = false;
                match result {
                    Ok(page) => {
                        let seen: std::collections::HashSet<String> =
                            p.items.iter().map(|i| i.id.clone()).collect();
                        let fresh: Vec<SeriesSummary> = page
                            .items
                            .into_iter()
                            .filter(|i| !seen.contains(&i.id))
                            .collect();
                        p.fetch_covers(
                            fresh
                                .iter()
                                .filter(|i| i.cover_url.is_empty())
                                .map(|i| i.id.clone())
                                .collect(),
                            cx,
                        );
                        p.items.extend(fresh);
                        p.cursor = page.cursor;
                        p.has_more = page.has_more && !p.cursor.is_empty();
                    }
                    Err(err) if err.is("END_OF_LIST") => p.has_more = false,
                    Err(err) => p.error = Some(err),
                }
                cx.notify();
            });
        }));
        cx.notify();
    }

    /// Search results carry no cover URL (the SDK's search parser has none),
    /// so they come from the series details.
    fn fetch_covers(&mut self, ids: Vec<String>, cx: &mut Context<Self>) {
        for chunk in ids.chunks(20) {
            let call =
                state::sdk(cx).call::<Vec<SeriesDetail>>("getSeriesBatch", (chunk.to_vec(),));
            self.cover_tasks.push(cx.spawn(async move |this, cx| {
                let result = call.await;
                let _ = this.update(cx, |p, cx| match result {
                    Ok(list) => {
                        for series in list.into_iter().filter_map(|d| d.series) {
                            if let Some(item) = p.items.iter_mut().find(|i| i.id == series.id) {
                                item.cover_url = series.cover_url;
                            }
                        }
                        cx.notify();
                    }
                    Err(err) => log::warn!("search covers: {err}"),
                });
            }));
        }
    }

    fn set_tab(&mut self, tab: usize, cx: &mut Context<Self>) {
        self.tab = tab;
        self.load(true, cx);
    }

    fn toggle_filter(&mut self, f: &'static str, cx: &mut Context<Self>) {
        if let Some(i) = self.filters.iter().position(|x| *x == f) {
            self.filters.remove(i);
        } else {
            self.filters.push(f);
        }
        self.load(true, cx);
    }

    fn maybe_more(&mut self, cx: &mut Context<Self>) {
        let (offset, max) = (self.scroll.offset().y, self.scroll.max_offset().y);
        if self.has_more && !self.loading && -offset >= max - px(700.) {
            self.load(false, cx);
        }
    }
}

impl Render for SearchPage {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        if self.has_more && !self.loading && self.filled_at != self.items.len() {
            self.filled_at = self.items.len();
            cx.on_next_frame(window, |this, _, cx| this.maybe_more(cx));
        }
        let layout = Layout::current(window, cx);
        let cols = layout::columns(window, cx, widgets::CARD_MIN);
        let p = palette(cx);
        let t = state::tr(cx);
        let zh = state::zh(cx);
        let field = div()
            .relative()
            .w_full()
            .max_w(px(720.))
            .child(
                div()
                    .flex()
                    .items_center()
                    .gap(px(10.))
                    .px(px(16.))
                    .h(px(48.))
                    .rounded(px(14.))
                    .bg(p.raised)
                    .border_1()
                    .border_color(p.line)
                    .child(icon(IconName::Search, 18., p.dim))
                    .child(
                        div()
                            .flex_1()
                            .child(Input::new(&self.input).appearance(false)),
                    ),
            )
            .when(!self.suggestions.is_empty(), |d| {
                d.child(
                    div()
                        .absolute()
                        .top(px(54.))
                        .left_0()
                        .right_0()
                        .py(px(6.))
                        .rounded(px(12.))
                        .bg(p.raised)
                        .border_1()
                        .border_color(p.line)
                        .shadow_lg()
                        .children(self.suggestions.iter().enumerate().map(|(i, s)| {
                            let s2 = s.clone();
                            div()
                                .id(("suggest", i))
                                .px(px(16.))
                                .py(px(8.))
                                .cursor_pointer()
                                .text_sm()
                                .text_color(p.fg)
                                .hover(|st| st.bg(p.hover))
                                .on_click(cx.listener(move |this, _, window, cx| {
                                    this.pick(s2.clone(), window, cx)
                                }))
                                .child(s.text.clone())
                        })),
                )
            });
        let tabs = div()
            .flex()
            .gap(px(8.))
            .children(t.search_tabs.iter().enumerate().map(|(i, label)| {
                widgets::chip(("search-tab", i), *label, i == self.tab, p)
                    .on_click(cx.listener(move |this, _, _, cx| this.set_tab(i, cx)))
            }));
        let filters =
            div()
                .flex()
                .flex_wrap()
                .gap(px(6.))
                .children(FILTERS.iter().enumerate().map(|(i, (id, z, e))| {
                    let on = self.filters.contains(id);
                    let id: &'static str = id;
                    div()
                        .id(("search-filter", i))
                        .px(px(11.))
                        .h(px(26.))
                        .flex()
                        .items_center()
                        .rounded_full()
                        .border_1()
                        .cursor_pointer()
                        .text_xs()
                        .when(on, |d| {
                            d.border_color(p.accent)
                                .bg(p.accent_soft)
                                .text_color(p.accent)
                        })
                        .when(!on, |d| {
                            d.border_color(p.line)
                                .text_color(p.dim)
                                .hover(|s| s.text_color(p.fg))
                        })
                        .on_click(cx.listener(move |this, _, _, cx| this.toggle_filter(id, cx)))
                        .child(if zh { *z } else { *e })
                }));
        let mut cards = Vec::with_capacity(self.items.len());
        for (i, item) in self.items.iter().enumerate() {
            let cover = state::cover(&item.cover_url, cx);
            let open = item.clone();
            let meta = if item.episode_count > 0 {
                fill(t.episodes, item.episode_count)
            } else {
                item.author.clone()
            };
            cards.push(
                Card::new(
                    ("search", i),
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
                widgets::ghost_button("search-retry", t.retry, None, p)
                    .on_click(cx.listener(|this, _, _, cx| this.load(this.items.is_empty(), cx))),
                p,
            )
            .into_any_element()
        } else if self.loading {
            widgets::centered(t.loading, p).into_any_element()
        } else if self.searched && self.items.is_empty() {
            widgets::centered(t.empty, p).into_any_element()
        } else if self.has_more {
            div()
                .w_full()
                .py(px(28.))
                .flex()
                .justify_center()
                .child(
                    widgets::ghost_button("search-more", t.load_more, None, p)
                        .on_click(cx.listener(|this, _, _, cx| this.load(false, cx))),
                )
                .into_any_element()
        } else {
            div().h(px(32.)).into_any_element()
        };
        div()
            .id("search")
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
                    .gap(px(18.))
                    .child(widgets::title(t.search, p))
                    .child(field)
                    .when(self.searched, |d| {
                        d.child(tabs).when(self.tab == 0, |d| d.child(filters))
                    })
                    .child(widgets::grid(cols).children(cards))
                    .child(footer),
            )
    }
}
