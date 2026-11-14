//! The panel beside the video: the series at the top, the episode grid below.

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::Theater;
use crate::sdk::types::Episode;
use crate::ui::i18n::{Tr, fill};
use crate::ui::icons::icon;
use crate::ui::state;
use crate::ui::theme::Palette;
use crate::ui::widgets;

pub const RAIL_W: f32 = 340.0;
const COLUMNS: u16 = 5;

impl Theater {
    pub(super) fn rail_panel(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> Div {
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
            .child(self.series_header(t, p, cx))
            .child(div().mx(px(18.)).h(px(1.)).bg(p.line))
            .child(
                div()
                    .px(px(18.))
                    .pt(px(16.))
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
                    .min_h_0()
                    .overflow_y_scroll()
                    .px(px(18.))
                    .pb(px(18.))
                    .child(div().grid().grid_cols(COLUMNS).gap(px(8.)).children(tiles)),
            )
    }

    fn series_header(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> Div {
        let series = &self.req.series;
        let cover = state::cover(&series.cover_url, cx);
        let mut line = fill(t.episode_n, self.current.number);
        if !self.req.episodes.is_empty() {
            line = format!("{line} / {}", fill(t.episodes, self.req.episodes.len()));
        }
        div()
            .p(px(18.))
            .flex()
            .gap(px(14.))
            .child(
                div()
                    .flex_none()
                    .w(px(72.))
                    .h(px(108.))
                    .rounded(px(10.))
                    .overflow_hidden()
                    .bg(p.raised)
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
                    .flex()
                    .flex_col()
                    .gap(px(6.))
                    .child(
                        div()
                            .text_base()
                            .font_weight(FontWeight::BOLD)
                            .line_clamp(2)
                            .child(series.title.clone()),
                    )
                    .child(div().text_xs().text_color(p.accent).child(line))
                    .when(!series.description.is_empty(), |d| {
                        d.child(
                            div()
                                .text_xs()
                                .text_color(p.dim)
                                .line_clamp(3)
                                .child(series.description.clone()),
                        )
                    }),
            )
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
        .h(px(40.))
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
