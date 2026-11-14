//! The bottom bar's buttons and the quality and speed menus that open above them.

use std::time::Duration;

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::{Menu, SPEEDS, Theater};
use crate::ui::icons::icon;
use crate::ui::theme::Palette;

pub fn speed_label(speed: f64) -> String {
    format!("{speed}x")
}

impl Theater {
    /// The pill that opens `menu`, with the menu above it while open.
    pub(super) fn menu_pill(
        &self,
        menu: Menu,
        label: SharedString,
        lit: bool,
        p: Palette,
        cx: &mut Context<Self>,
    ) -> Div {
        let open = self.menu == Some(menu);
        let (id, list) = match menu {
            Menu::Quality => ("t-quality", self.quality_menu(p, cx)),
            Menu::Speed => ("t-speed", self.speed_menu(p, cx)),
        };
        div()
            .relative()
            .child(
                pill_button(id, label, None, open || lit, p)
                    .on_click(cx.listener(move |this, _, _, cx| this.toggle_menu(menu, cx))),
            )
            .when(open, |d| {
                d.child(div().absolute().right_0().child(list).with_animation(
                    id,
                    Animation::new(Duration::from_millis(180)).with_easing(ease_out_quint()),
                    |el, t| el.opacity(t).bottom(px(40. - 10. * (1. - t))),
                ))
            })
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
        menu_list(p).children(streams.into_iter().rev().enumerate().map(|(i, s)| {
            let key = if s.definition.is_empty() {
                s.quality.clone()
            } else {
                s.definition.clone()
            };
            menu_item(
                ("t-q", i),
                s.label(),
                s.codec.to_uppercase(),
                s.url == current,
                p,
            )
            .on_click(
                cx.listener(move |this, _, window, cx| this.set_quality(key.clone(), window, cx)),
            )
        }))
    }

    fn speed_menu(&self, p: Palette, cx: &mut Context<Self>) -> Div {
        let now = self.speed(cx);
        menu_list(p).children(SPEEDS.iter().rev().enumerate().map(|(i, &s)| {
            menu_item(("t-s", i), speed_label(s), String::new(), s == now, p)
                .on_click(cx.listener(move |this, _, window, cx| this.set_speed(s, window, cx)))
        }))
    }
}

fn menu_list(p: Palette) -> Div {
    div()
        .occlude()
        .min_w(px(132.))
        .p(px(6.))
        .rounded(px(12.))
        .bg(p.raised.opacity(0.96))
        .border_1()
        .border_color(p.line)
        .shadow_lg()
        .flex()
        .flex_col()
}

fn menu_item(
    id: (&'static str, usize),
    label: String,
    detail: String,
    on: bool,
    p: Palette,
) -> Stateful<Div> {
    div()
        .id(id)
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
        .child(label)
        .child(div().text_xs().text_color(p.faint).child(detail))
}

pub fn control(id: &'static str, ic: IconName, p: Palette) -> Stateful<Div> {
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

pub fn pill_button(
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
