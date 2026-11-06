//! Shared motion: one easing, one spring, and an enter transition. Every
//! animation here is oneshot or spring-settled, so nothing redraws once it
//! rests, and GPUI skips them all when `App::reduce_motion` is set.

use std::time::Duration;

use gpui_kit::*;

/// Hover lift and the sidebar pill: quick, with a little overshoot.
pub const SNAPPY: SpringConfig = SpringConfig::new(420., 26., 1.);

pub fn spring() -> SpringAnimation {
    SpringAnimation::new(SNAPPY)
}

/// Fades `el` in while it rises `dy` pixels into place, after `delay`.
/// The id decides when it replays: a new id is a new entrance.
pub fn enter<E: Styled + IntoElement + 'static>(
    el: E,
    id: impl Into<ElementId>,
    ms: u64,
    delay: Duration,
    dy: f32,
) -> AnimationElement<E> {
    // Always two steps: GPUI keeps the step index under the id, so an element
    // whose delay drops to zero (a card moving to the front) must not lose one.
    let steps = vec![
        Animation::new(delay.max(Duration::from_millis(1))),
        Animation::new(Duration::from_millis(ms)).with_easing(ease_out_quint()),
    ];
    el.with_animations(id, steps, move |el, step, t| {
        let t = if step == 0 { 0. } else { t };
        if dy == 0. {
            el.opacity(t)
        } else {
            el.opacity(t).relative().top(px(dy * (1. - t)))
        }
    })
}

/// The entrance delay for the `order`th item of a grid, so a screen of cards
/// arrives as a quick wave instead of all at once.
pub fn stagger(order: usize) -> Duration {
    Duration::from_millis(order.min(14) as u64 * 22)
}
