use std::collections::HashMap;
use std::sync::Arc;

use anyhow::{Context as _, Result};
use gpui_kit::*;
use ohg_desktop::sdk::Sdk;
use ohg_desktop::settings::{self, Settings};
use ohg_desktop::ui::app::AppView;
use ohg_desktop::ui::cover::Covers;
use ohg_desktop::ui::icons::AppAssets;
use ohg_desktop::ui::state::{self, Ohg};
use ohg_desktop::ui::theme;

fn main() -> Result<()> {
    env_logger::init();
    let dir = settings::config_dir()?;
    std::fs::create_dir_all(&dir).with_context(|| format!("create {}", dir.display()))?;
    let settings = Settings::load(&dir)?;
    let sdk = Arc::new(Sdk::open(&dir, "").map_err(|e| anyhow::anyhow!("open SDK: {e}"))?);
    let covers = Covers::new()?;
    gpui_kit::application()
        .with_assets(AppAssets)
        .run(move |cx| {
            gpui_kit::init(cx);
            cx.bind_keys([KeyBinding::new(
                "f12",
                ohg_desktop::ui::debug::ToggleDebug,
                None,
            )]);
            let size = size(px(settings.window.width), px(settings.window.height));
            let covers = cx.new(|_| covers);
            cx.set_global(Ohg {
                sdk,
                covers,
                settings,
                config_dir: dir,
                history: HashMap::new(),
            });
            state::reload_history(cx).detach();
            let options = WindowOptions {
                window_bounds: Some(WindowBounds::Windowed(Bounds::centered(None, size, cx))),
                app_id: Some("openhongguo".into()),
                titlebar: Some(TitlebarOptions {
                    title: Some("红果短剧".into()),
                    ..Default::default()
                }),
                window_min_size: Some(gpui_kit::size(px(860.), px(560.))),
                ..Default::default()
            };
            let opened = gpui_kit::open_window(options, cx, |window, cx| {
                let theme = state::ohg(cx).settings.theme.clone();
                theme::apply(&theme, window, cx);
                cx.new(|cx| AppView::new(window, cx))
            });
            if let Err(err) = opened {
                eprintln!("ohg-desktop: open window: {err:#}");
                cx.quit();
            }
        });
    Ok(())
}
