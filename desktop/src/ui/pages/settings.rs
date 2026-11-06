//! Language, theme, quality, autoplay, device reset, and the key map.

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use crate::settings::Settings;
use crate::ui::i18n::Tr;
use crate::ui::layout::Layout;
use crate::ui::state::{self, Nav};
use crate::ui::theme::{Palette, palette};
use crate::ui::widgets;

const QUALITIES: [&str; 5] = ["best", "1080p", "720p", "540p", "worst"];

#[derive(Default)]
pub struct SettingsPage {
    resetting: bool,
    task: Option<Task<()>>,
}

impl EventEmitter<Nav> for SettingsPage {}

impl SettingsPage {
    fn set(
        &mut self,
        window: &mut Window,
        cx: &mut Context<Self>,
        change: impl FnOnce(&mut Settings),
    ) {
        match state::update_settings(window, cx, change) {
            Ok(()) => cx.emit(Nav::toast(state::tr(cx).saved)),
            Err(err) => cx.emit(Nav::error(format!("{err:#}"))),
        }
    }

    fn reset_device(&mut self, cx: &mut Context<Self>) {
        self.resetting = true;
        let call = state::sdk(cx).call::<serde_json::Value>("resetDevice", ());
        self.task = Some(cx.spawn(async move |this, cx| {
            let result = call.await;
            let _ = this.update(cx, |p, cx| {
                p.resetting = false;
                match result {
                    Ok(_) => cx.emit(Nav::toast(state::tr(cx).device_reset)),
                    Err(err) => cx.emit(Nav::error(err.describe(state::zh(cx)))),
                }
                cx.notify();
            });
        }));
        cx.notify();
    }
}

impl Render for SettingsPage {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let pad = Layout::current(window, cx).pad();
        let p = palette(cx);
        let t = state::tr(cx);
        let s = state::ohg(cx).settings.clone();
        let language = choices(
            "lang",
            &[("zh", "中文"), ("en", "English")],
            &s.language,
            p,
            cx,
            |this, v, window, cx| this.set(window, cx, |s| s.language = v.into()),
        );
        let theme = choices(
            "theme",
            &[
                ("auto", t.theme_auto),
                ("dark", t.theme_dark),
                ("light", t.theme_light),
            ],
            &s.theme,
            p,
            cx,
            |this, v, window, cx| this.set(window, cx, |s| s.theme = v.into()),
        );
        let quality_items: Vec<(&'static str, &'static str)> = QUALITIES
            .iter()
            .map(|q| {
                (
                    *q,
                    if *q == "best" {
                        if state::zh(cx) { "最高" } else { "Best" }
                    } else if *q == "worst" {
                        if state::zh(cx) { "最低" } else { "Lowest" }
                    } else {
                        q
                    },
                )
            })
            .collect();
        let quality = choices(
            "quality",
            &quality_items,
            &s.quality,
            p,
            cx,
            |this, v, window, cx| this.set(window, cx, |s| s.quality = v.into()),
        );
        let autoplay = choices(
            "autoplay",
            &[("on", t.on), ("off", t.off)],
            if s.autoplay { "on" } else { "off" },
            p,
            cx,
            |this, v, window, cx| this.set(window, cx, |s| s.autoplay = v == "on"),
        );
        let debug = choices(
            "debug",
            &[("on", t.on), ("off", t.off)],
            if s.debug { "on" } else { "off" },
            p,
            cx,
            |this, v, window, cx| this.set(window, cx, |s| s.debug = v == "on"),
        );
        let reset = widgets::ghost_button(
            "reset-device",
            t.set_reset_device,
            Some(IconName::RefreshCw),
            p,
        )
        .when(!self.resetting, |b| {
            b.on_click(cx.listener(|this, _, _, cx| this.reset_device(cx)))
        })
        .when(self.resetting, |b| b.opacity(0.5));
        div().id("settings").size_full().overflow_y_scroll().child(
            div()
                .px(px(pad))
                .pt(px(28.))
                .pb(px(40.))
                .max_w(px(820.))
                .flex()
                .flex_col()
                .gap(px(8.))
                .child(div().pb(px(14.)).child(widgets::title(t.settings, p)))
                .child(row(t.set_language, None, language, p))
                .child(row(t.set_theme, None, theme, p))
                .child(row(t.set_quality, None, quality, p))
                .child(row(t.set_autoplay, None, autoplay, p))
                .child(row(t.set_reset_device, Some(t.set_reset_hint), reset, p))
                .child(row(t.set_debug, Some(t.set_debug_hint), debug, p))
                .child(div().pt(px(26.)).child(keys(t, p))),
        )
    }
}

fn row(
    label: &'static str,
    hint: Option<&'static str>,
    control: impl IntoElement,
    p: Palette,
) -> Div {
    div()
        .flex()
        .flex_wrap()
        .items_center()
        .justify_between()
        .gap_x(px(24.))
        .gap_y(px(10.))
        .py(px(14.))
        .border_b_1()
        .border_color(p.line)
        .child(
            div()
                .flex()
                .flex_col()
                .gap(px(3.))
                .child(
                    div()
                        .text_sm()
                        .font_weight(FontWeight::MEDIUM)
                        .text_color(p.fg)
                        .child(label),
                )
                .when_some(hint, |d, h| {
                    d.child(div().text_xs().text_color(p.dim).child(h))
                }),
        )
        .child(control)
}

fn choices(
    id: &'static str,
    items: &[(&'static str, &'static str)],
    current: &str,
    p: Palette,
    cx: &mut Context<SettingsPage>,
    on_pick: impl Fn(&mut SettingsPage, &'static str, &mut Window, &mut Context<SettingsPage>)
    + Clone
    + 'static,
) -> Div {
    div()
        .flex()
        .p(px(3.))
        .gap(px(2.))
        .rounded_full()
        .bg(p.raised)
        .border_1()
        .border_color(p.line)
        .children(items.iter().enumerate().map(|(i, (value, label))| {
            let on = *value == current;
            let value: &'static str = value;
            let pick = on_pick.clone();
            div()
                .id((id, i))
                .px(px(14.))
                .h(px(28.))
                .flex()
                .items_center()
                .rounded_full()
                .cursor_pointer()
                .text_sm()
                .when(on, |d| d.bg(p.accent).text_color(p.accent_fg))
                .when(!on, |d| d.text_color(p.dim).hover(|s| s.text_color(p.fg)))
                .on_click(cx.listener(move |this, _, window, cx| {
                    if !on {
                        pick(this, value, window, cx)
                    }
                }))
                .child(*label)
        }))
}

fn keys(t: &Tr, p: Palette) -> Div {
    div()
        .flex()
        .flex_col()
        .gap(px(10.))
        .child(widgets::subtitle(t.keys_title, p))
        .children(t.keys.iter().map(|(k, what)| {
            div()
                .flex()
                .items_center()
                .gap(px(14.))
                .child(
                    div()
                        .min_w(px(78.))
                        .px(px(8.))
                        .py(px(3.))
                        .rounded(px(6.))
                        .bg(p.raised)
                        .border_1()
                        .border_color(p.line)
                        .text_xs()
                        .text_color(p.fg)
                        .child(*k),
                )
                .child(div().text_sm().text_color(p.dim).child(*what))
        }))
}
