//! Small building blocks shared by the pages.

use std::sync::Arc;

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::icons::icon;
use super::motion;
use super::theme::{Palette, palette};

pub const GAP_X: f32 = 20.0;
/// Covers grow from this width until another column fits.
pub const CARD_MIN: f32 = 176.0;

type OnClick = Box<dyn Fn(&ClickEvent, &mut Window, &mut App) + 'static>;

/// A portrait 2:3 cover with a title under it. It rises into place when it
/// first shows a series and lifts on a spring under the pointer.
#[derive(IntoElement)]
pub struct Card {
    pub id: ElementId,
    pub title: SharedString,
    pub meta: SharedString,
    pub badge: Option<SharedString>,
    /// 0..1 drawn as a bar along the bottom of the cover.
    pub progress: Option<f32>,
    pub cover: Option<Arc<RenderImage>>,
    /// Position in its grid, for the entrance stagger.
    pub order: usize,
    pub on_click: OnClick,
}

impl Card {
    pub fn new(
        id: impl Into<ElementId>,
        title: impl Into<SharedString>,
        cover: Option<Arc<RenderImage>>,
        on_click: impl Fn(&ClickEvent, &mut Window, &mut App) + 'static,
    ) -> Card {
        Card {
            id: id.into(),
            title: title.into(),
            meta: SharedString::default(),
            badge: None,
            progress: None,
            cover,
            order: 0,
            on_click: Box::new(on_click),
        }
    }

    pub fn meta(mut self, meta: impl Into<SharedString>) -> Card {
        self.meta = meta.into();
        self
    }

    pub fn badge(mut self, badge: impl Into<SharedString>) -> Card {
        let b: SharedString = badge.into();
        self.badge = (!b.is_empty()).then_some(b);
        self
    }

    pub fn progress(mut self, progress: Option<f32>) -> Card {
        self.progress = progress;
        self
    }

    pub fn order(mut self, order: usize) -> Card {
        self.order = order;
        self
    }
}

impl RenderOnce for Card {
    fn render(self, window: &mut Window, cx: &mut App) -> impl IntoElement {
        let p = palette(cx);
        let id = Arc::new(self.id.clone());
        let hovered = window.use_keyed_state(
            ElementId::NamedChild(id.clone(), "hover".into()),
            cx,
            |_, _| false,
        );
        let lifted = *hovered.read(cx);
        let group: SharedString = format!("card-{:?}", self.id).into();
        let poster = div()
            .relative()
            .w_full()
            .aspect_ratio(2. / 3.)
            .rounded(px(12.))
            .overflow_hidden()
            .bg(p.raised)
            .border_1()
            .border_color(p.line)
            .group_hover(group.clone(), |s| s.border_color(p.accent))
            .child(match self.cover {
                Some(cover) => motion::enter(
                    img(ImageSource::Render(cover))
                        .size_full()
                        .object_fit(ObjectFit::Cover),
                    ElementId::NamedChild(id.clone(), "cover".into()),
                    320,
                    std::time::Duration::ZERO,
                    0.,
                )
                .into_any_element(),
                None => placeholder(&self.title, p).into_any_element(),
            })
            .child(
                div()
                    .absolute()
                    .inset_0()
                    .flex()
                    .items_center()
                    .justify_center()
                    .bg(p.scrim)
                    .opacity(0.)
                    .group_hover(group.clone(), |s| s.opacity(1.))
                    .child(
                        div()
                            .size(px(48.))
                            .rounded_full()
                            .bg(p.accent)
                            .flex()
                            .items_center()
                            .justify_center()
                            .child(icon(IconName::Play, 22., p.accent_fg)),
                    ),
            )
            .when_some(self.badge, |d, b| {
                d.child(
                    div()
                        .absolute()
                        .top(px(8.))
                        .right(px(8.))
                        .px(px(7.))
                        .py(px(2.))
                        .rounded(px(6.))
                        .bg(hsla(0., 0., 0., 0.6))
                        .text_color(white())
                        .text_xs()
                        .child(b),
                )
            })
            .when_some(self.progress, |d, f| {
                d.child(
                    div()
                        .absolute()
                        .bottom_0()
                        .left_0()
                        .right_0()
                        .h(px(3.))
                        .bg(hsla(0., 0., 0., 0.5))
                        .child(div().h_full().w(relative(f.clamp(0., 1.))).bg(p.accent)),
                )
            })
            .with_spring(
                ElementId::NamedChild(id.clone(), "lift".into()),
                motion::spring().to(lifted),
                move |el, phase| {
                    let t = phase.0;
                    el.top(px(-6. * t)).shadow(vec![BoxShadow {
                        color: hsla(0., 0., 0., 0.45 * t.clamp(0., 1.)),
                        offset: point(px(0.), px(14. * t)),
                        blur_radius: px(28. * t),
                        spread_radius: px(0.),
                        inset: false,
                    }])
                },
            );
        let entrance = ElementId::NamedChild(id, self.title.clone());
        let card = div()
            .id(self.id)
            .group(group)
            .w_full()
            .min_w_0()
            .flex()
            .flex_col()
            .gap(px(8.))
            .cursor_pointer()
            .on_hover(move |h, _, cx| {
                hovered.update(cx, |v, cx| {
                    *v = *h;
                    cx.notify();
                })
            })
            .on_click(self.on_click)
            .child(poster)
            .child(
                div()
                    .text_sm()
                    .font_weight(FontWeight::MEDIUM)
                    .text_color(p.fg)
                    .line_clamp(2)
                    .child(self.title),
            )
            .when(!self.meta.is_empty(), |d| {
                d.child(
                    div()
                        .text_xs()
                        .text_color(p.dim)
                        .truncate()
                        .child(self.meta),
                )
            });
        // Keyed by title too, so a grid refilled with other series replays it.
        motion::enter(card, entrance, 420, motion::stagger(self.order), 14.)
    }
}

fn placeholder(title: &str, p: Palette) -> Div {
    let initial: String = title.chars().next().map(String::from).unwrap_or_default();
    div()
        .size_full()
        .flex()
        .items_center()
        .justify_center()
        .text_3xl()
        .text_color(p.faint)
        .child(initial)
}

/// How many `min`-wide columns fit the page area beside the sidebar.
pub fn grid(cols: u16) -> Div {
    div().grid().grid_cols(cols).gap_x(px(GAP_X)).gap_y(px(28.))
}

pub fn pill(text: impl Into<SharedString>, p: Palette) -> Div {
    div()
        .px(px(10.))
        .py(px(4.))
        .rounded_full()
        .bg(p.raised)
        .border_1()
        .border_color(p.line)
        .text_xs()
        .text_color(p.dim)
        .child(text.into())
}

pub fn title(text: impl Into<SharedString>, p: Palette) -> Div {
    div()
        .text_2xl()
        .font_weight(FontWeight::SEMIBOLD)
        .text_color(p.fg)
        .child(text.into())
}

pub fn subtitle(text: impl Into<SharedString>, p: Palette) -> Div {
    div()
        .text_base()
        .font_weight(FontWeight::SEMIBOLD)
        .text_color(p.fg)
        .child(text.into())
}

pub fn primary_button(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    ic: Option<IconName>,
    p: Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .flex()
        .items_center()
        .gap(px(8.))
        .px(px(20.))
        .h(px(40.))
        .rounded_full()
        .bg(p.accent)
        .hover(|s| s.bg(p.accent.opacity(0.88)))
        .active(|s| s.bg(p.accent.opacity(0.75)))
        .cursor_pointer()
        .text_sm()
        .font_weight(FontWeight::SEMIBOLD)
        .text_color(p.accent_fg)
        .when_some(ic, |d, ic| d.child(icon(ic, 16., p.accent_fg)))
        .child(label.into())
}

pub fn ghost_button(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    ic: Option<IconName>,
    p: Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .flex()
        .items_center()
        .gap(px(8.))
        .px(px(16.))
        .h(px(36.))
        .rounded_full()
        .bg(p.raised)
        .border_1()
        .border_color(p.line)
        .hover(|s| s.bg(p.hover))
        .cursor_pointer()
        .text_sm()
        .text_color(p.fg)
        .when_some(ic, |d, ic| d.child(icon(ic, 15., p.fg)))
        .child(label.into())
}

/// A segmented row of choices; `selected` is highlighted.
pub fn chip(
    id: impl Into<ElementId>,
    label: impl Into<SharedString>,
    selected: bool,
    p: Palette,
) -> Stateful<Div> {
    div()
        .id(id)
        .px(px(14.))
        .h(px(32.))
        .flex()
        .items_center()
        .rounded_full()
        .cursor_pointer()
        .text_sm()
        .when(selected, |d| {
            d.bg(p.fg).text_color(p.bg).font_weight(FontWeight::MEDIUM)
        })
        .when(!selected, |d| {
            d.bg(p.raised)
                .text_color(p.dim)
                .hover(|s| s.bg(p.hover).text_color(p.fg))
        })
        .child(label.into())
}

pub fn centered(text: impl Into<SharedString>, p: Palette) -> Div {
    div()
        .w_full()
        .py(px(80.))
        .flex()
        .justify_center()
        .text_sm()
        .text_color(p.dim)
        .child(text.into())
}

/// A message with a retry button the caller wires up.
pub fn error_state(message: impl Into<SharedString>, retry: impl IntoElement, p: Palette) -> Div {
    div()
        .w_full()
        .py(px(80.))
        .flex()
        .flex_col()
        .items_center()
        .gap(px(14.))
        .child(icon(IconName::TriangleAlert, 28., p.orange))
        .child(div().text_sm().text_color(p.dim).child(message.into()))
        .child(retry)
}
