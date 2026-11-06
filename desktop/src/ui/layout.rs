//! Breakpoints. The window may be a narrow tiling column, a half screen or a
//! full 4K monitor, so the frame (navigation, padding, card size) follows the
//! width the content actually gets.

use gpui_kit::*;

use super::widgets::GAP_X;
use super::{debug, state};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Layout {
    /// Phone-like: bottom tab bar, tight padding, smaller cards.
    Compact,
    /// Icon-only sidebar.
    Medium,
    /// Full sidebar with labels.
    Wide,
}

impl Layout {
    pub fn of(width: f32) -> Layout {
        if width < 720. {
            Layout::Compact
        } else if width < 1100. {
            Layout::Medium
        } else {
            Layout::Wide
        }
    }

    pub fn current(window: &Window, cx: &App) -> Layout {
        Layout::of(usable_width(window, cx))
    }

    pub fn sidebar_w(self) -> f32 {
        match self {
            Layout::Wide => 216.,
            Layout::Medium => 76.,
            Layout::Compact => 0.,
        }
    }

    pub fn pad(self) -> f32 {
        match self {
            Layout::Wide => 36.,
            Layout::Medium => 28.,
            Layout::Compact => 16.,
        }
    }

    /// Cards shrink by a quarter at phone width, as in the phone app's grid.
    fn card_scale(self) -> f32 {
        if self == Layout::Compact { 0.75 } else { 1. }
    }
}

/// The window width left beside the docked debug panel.
pub fn usable_width(window: &Window, cx: &App) -> f32 {
    let docked = if state::ohg(cx).settings.debug {
        debug::PANEL_W
    } else {
        0.
    };
    f32::from(window.viewport_size().width) - docked
}

/// How many cards of at least `min` pixels fit across the page.
pub fn columns(window: &Window, cx: &App, min: f32) -> u16 {
    columns_for(usable_width(window, cx), min)
}

fn columns_for(width: f32, min: f32) -> u16 {
    let layout = Layout::of(width);
    let min = min * layout.card_scale();
    let inner = width - layout.sidebar_w() - 2. * layout.pad();
    (((inner + GAP_X) / (min + GAP_X)).floor() as u16).max(2)
}

#[cfg(test)]
mod tests {
    use super::{Layout, columns_for};

    #[test]
    fn breakpoints_and_columns_follow_the_width() {
        assert_eq!(Layout::of(480.), Layout::Compact);
        assert_eq!(Layout::of(853.), Layout::Medium);
        assert_eq!(Layout::of(1280.), Layout::Wide);
        assert_eq!(columns_for(2560., 176.), 11);
        assert_eq!(columns_for(1280., 176.), 5);
        assert_eq!(columns_for(853., 176.), 3);
        assert_eq!(columns_for(480., 176.), 3);
        assert_eq!(columns_for(300., 176.), 2);
    }
}
