//! Navigation: the sidebar, labeled when wide and icons only when medium,
//! and the tab bar that replaces it at phone width.

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::{AppView, Section};
use crate::ui::i18n::Tr;
use crate::ui::icons::icon;
use crate::ui::layout::Layout;
use crate::ui::motion;
use crate::ui::theme::Palette;

impl AppView {
    /// The highlighted section; none while a series page is on top.
    fn active(&self) -> Option<Section> {
        self.stack.is_empty().then_some(self.section)
    }

    /// Medium windows get the same sidebar without labels.
    pub(super) fn sidebar(
        &self,
        t: &'static Tr,
        p: Palette,
        layout: Layout,
        cx: &mut Context<Self>,
    ) -> Div {
        let [a, b, c, d, settings_item] = nav(t);
        let items = [a, b, c, d];
        let labeled = layout == Layout::Wide;
        let active = items.iter().position(|(s, _, _)| Some(*s) == self.active());
        let settings_on = self.active() == Some(Section::Settings);
        div()
            .w(px(layout.sidebar_w()))
            .h_full()
            .flex_none()
            .flex()
            .flex_col()
            .bg(p.sidebar)
            .border_r_1()
            .border_color(p.line)
            .child(
                div()
                    .px(px(if labeled { 20. } else { 23. }))
                    .pt(px(22.))
                    .pb(px(26.))
                    .flex()
                    .items_center()
                    .gap(px(10.))
                    .child(
                        div()
                            .size(px(30.))
                            .rounded(px(9.))
                            .bg(p.accent)
                            .flex()
                            .items_center()
                            .justify_center()
                            .child(icon(IconName::Clapperboard, 16., p.accent_fg)),
                    )
                    .when(labeled, |d| {
                        d.child(
                            div()
                                .text_base()
                                .font_weight(FontWeight::BOLD)
                                .text_color(p.fg)
                                .child(t.app),
                        )
                    }),
            )
            .child(
                div()
                    .px(px(10.))
                    .relative()
                    .flex()
                    .flex_col()
                    .gap(px(NAV_GAP))
                    .when_some(active, |d, i| d.child(nav_pill(i, p)))
                    .children(items.into_iter().map(|(s, ic, label)| {
                        nav_item(s, ic, label, Some(s) == self.active(), labeled, p, cx)
                    })),
            )
            .child(div().flex_1())
            .child(
                div()
                    .px(px(10.))
                    .pb(px(14.))
                    .relative()
                    .when(settings_on, |d| d.child(nav_pill(0, p)))
                    .child(nav_item(
                        settings_item.0,
                        settings_item.1,
                        settings_item.2,
                        settings_on,
                        labeled,
                        p,
                        cx,
                    )),
            )
    }

    /// Compact windows navigate like the phone app, from a bottom tab bar.
    pub(super) fn tab_bar(&self, t: &'static Tr, p: Palette, cx: &mut Context<Self>) -> Div {
        let items = nav(t);
        let n = items.len() as f32;
        let active = items.iter().position(|(s, _, _)| Some(*s) == self.active());
        div()
            .relative()
            .flex_none()
            .h(px(TAB_H))
            .flex()
            .bg(p.sidebar)
            .border_t_1()
            .border_color(p.line)
            .when_some(active, |d, i| {
                d.child(
                    div()
                        .absolute()
                        .top_0()
                        .w(relative(1. / n))
                        .flex()
                        .justify_center()
                        .child(div().w(px(28.)).h(px(3.)).rounded_full().bg(p.accent))
                        .with_spring("tab-pill", motion::spring().to(i as f32), move |el, x| {
                            el.left(relative(x / n))
                        }),
                )
            })
            .children(items.into_iter().map(|(section, ic, label)| {
                let on = Some(section) == self.active();
                let color = if on { p.accent } else { p.dim };
                div()
                    .id(label)
                    .flex_1()
                    .flex()
                    .flex_col()
                    .items_center()
                    .justify_center()
                    .gap(px(3.))
                    .cursor_pointer()
                    .text_xs()
                    .text_color(color)
                    .when(on, |d| d.font_weight(FontWeight::SEMIBOLD))
                    .child(icon(ic, 20., color))
                    .child(label)
                    .on_click(cx.listener(move |this, _, window, cx| this.go(section, window, cx)))
            }))
    }
}

const TAB_H: f32 = 60.0;

fn nav(t: &'static Tr) -> [(Section, IconName, &'static str); 5] {
    [
        (Section::Continue, IconName::House, t.continue_),
        (Section::Discover, IconName::Compass, t.discover),
        (Section::Search, IconName::Search, t.search),
        (Section::History, IconName::RotateCcwClock, t.history),
        (Section::Settings, IconName::Settings, t.settings),
    ]
}

const NAV_H: f32 = 38.0;
const NAV_GAP: f32 = 2.0;

/// The active item's highlight, drawn under the items. It springs to the
/// new row when the section changes.
fn nav_pill(row: usize, p: Palette) -> impl IntoElement {
    div()
        .absolute()
        .left(px(10.))
        .right(px(10.))
        .h(px(NAV_H))
        .rounded(px(10.))
        .bg(p.accent_soft)
        .with_spring(
            "nav-pill",
            motion::spring().to(px(row as f32 * (NAV_H + NAV_GAP))),
            |el, y| el.top(y),
        )
}

fn nav_item(
    section: Section,
    ic: IconName,
    label: &'static str,
    on: bool,
    labeled: bool,
    p: Palette,
    cx: &mut Context<AppView>,
) -> Stateful<Div> {
    div()
        .id(label)
        .h(px(NAV_H))
        .px(px(12.))
        .flex()
        .items_center()
        .when(!labeled, |d| d.justify_center())
        .gap(px(12.))
        .rounded(px(10.))
        .cursor_pointer()
        .text_sm()
        .when(on, |d| d.text_color(p.fg).font_weight(FontWeight::SEMIBOLD))
        .when(!on, |d| {
            d.text_color(p.dim)
                .hover(|s| s.bg(p.hover).text_color(p.fg))
        })
        .child(icon(ic, 17., if on { p.accent } else { p.dim }))
        .when(labeled, |d| d.child(label))
        .on_click(cx.listener(move |this, _, window, cx| this.go(section, window, cx)))
}
