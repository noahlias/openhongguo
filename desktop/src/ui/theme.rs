//! The app's own palette, carried over from `internal/tui/theme.go`, plus the
//! few gpui-component colors its inputs and sliders read.

use gpui_kit::component::{Theme, ThemeMode};
use gpui_kit::*;

#[derive(Clone, Copy)]
pub struct Palette {
    pub dark: bool,
    pub bg: Hsla,
    pub sidebar: Hsla,
    pub raised: Hsla,
    pub hover: Hsla,
    pub line: Hsla,
    pub fg: Hsla,
    pub dim: Hsla,
    pub faint: Hsla,
    pub accent: Hsla,
    pub accent_soft: Hsla,
    pub accent_fg: Hsla,
    pub orange: Hsla,
    pub green: Hsla,
    pub gold: Hsla,
    pub scrim: Hsla,
}

impl Global for Palette {}

fn hex(v: u32) -> Hsla {
    rgb(v).into()
}

fn hexa(v: u32, alpha: f32) -> Hsla {
    let mut c = hex(v);
    c.a = alpha;
    c
}

impl Palette {
    pub fn dark() -> Palette {
        Palette {
            dark: true,
            bg: hex(0x0E0E10),
            sidebar: hex(0x131315),
            raised: hex(0x1C1C1F),
            hover: hex(0x26262A),
            line: hex(0x2A2A2E),
            fg: hex(0xECECEE),
            dim: hex(0x8E8E93),
            faint: hex(0x5A5A60),
            accent: hex(0xE5484D),
            accent_soft: hexa(0xE5484D, 0.16),
            accent_fg: hex(0xFFFFFF),
            orange: hex(0xF76B15),
            green: hex(0x30A46C),
            gold: hex(0xF5B400),
            scrim: hexa(0x000000, 0.62),
        }
    }

    pub fn light() -> Palette {
        Palette {
            dark: false,
            bg: hex(0xF7F7F8),
            sidebar: hex(0xEFEFF1),
            raised: hex(0xFFFFFF),
            hover: hex(0xE8E8EB),
            line: hex(0xDDDDE2),
            fg: hex(0x1F1F1F),
            dim: hex(0x6F6F6F),
            faint: hex(0xA0A0A6),
            accent: hex(0xE5484D),
            accent_soft: hexa(0xE5484D, 0.12),
            accent_fg: hex(0xFFFFFF),
            orange: hex(0xF76B15),
            green: hex(0x30A46C),
            gold: hex(0xC98F00),
            scrim: hexa(0x000000, 0.45),
        }
    }
}

pub fn palette(cx: &App) -> Palette {
    *cx.global::<Palette>()
}

/// "auto" follows the window's appearance.
pub fn apply(theme: &str, window: &Window, cx: &mut App) {
    let dark = match theme {
        "light" => false,
        "dark" => true,
        _ => matches!(
            window.appearance(),
            WindowAppearance::Dark | WindowAppearance::VibrantDark
        ),
    };
    let p = if dark {
        Palette::dark()
    } else {
        Palette::light()
    };
    cx.set_global(p);
    Theme::change(
        if dark {
            ThemeMode::Dark
        } else {
            ThemeMode::Light
        },
        None,
        cx,
    );
    let font = ui_font(cx);
    let t = Theme::global_mut(cx);
    if let Some(font) = font {
        t.font_family = font.into();
    }
    t.background = p.bg;
    t.foreground = p.fg;
    t.border = p.line;
    t.input = p.line;
    t.ring = p.accent;
    t.caret = p.accent;
    t.primary = p.accent;
    t.primary_hover = p.accent.opacity(0.9);
    t.primary_active = p.accent.opacity(0.8);
    t.primary_foreground = p.accent_fg;
    t.slider_bar = p.accent;
    t.slider_thumb = p.accent_fg;
    t.switch = p.line;
    t.muted_foreground = p.dim;
    t.popover = p.raised;
    t.popover_foreground = p.fg;
    t.selection = p.accent_soft;
}

/// Noto Sans CJK SC, then LXGW WenKai; otherwise the toolkit's system font.
fn ui_font(cx: &App) -> Option<&'static str> {
    let names = cx.text_system().all_font_names();
    ["Noto Sans CJK SC", "LXGW WenKai"]
        .into_iter()
        .find(|f| names.iter().any(|n| n == f))
}
