//! App-wide state every view reads, and the events pages send to the shell.

use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::Arc;

use gpui_kit::*;

use super::cover::Covers;
use super::i18n::{self, Tr};
use crate::sdk::Sdk;
use crate::sdk::types::{Episode, HistoryEntry, SeriesSummary};
use crate::settings::Settings;

pub struct Ohg {
    pub sdk: Arc<Sdk>,
    pub covers: Entity<Covers>,
    pub settings: Settings,
    pub config_dir: PathBuf,
    /// `history.json` by series ID, refreshed after every write.
    pub history: HashMap<String, HistoryEntry>,
}

impl Global for Ohg {}

pub fn ohg(cx: &App) -> &Ohg {
    cx.global::<Ohg>()
}

pub fn tr(cx: &App) -> &'static Tr {
    i18n::tr(&ohg(cx).settings.language)
}

pub fn zh(cx: &App) -> bool {
    ohg(cx).settings.zh()
}

pub fn sdk(cx: &App) -> Arc<Sdk> {
    ohg(cx).sdk.clone()
}

pub fn covers(cx: &App) -> Entity<Covers> {
    ohg(cx).covers.clone()
}

/// Asks the cover cache for `url`; `None` while it loads.
pub fn cover(url: &str, cx: &mut App) -> Option<Arc<RenderImage>> {
    covers(cx).update(cx, |c, cx| c.get(url, cx))
}

/// Changes the settings, writes `desktop.json` and re-applies the theme.
pub fn update_settings(
    window: &mut Window,
    cx: &mut App,
    change: impl FnOnce(&mut Settings),
) -> anyhow::Result<()> {
    let (settings, dir) = cx.update_global::<Ohg, _>(|o, _| {
        change(&mut o.settings);
        (o.settings.clone(), o.config_dir.clone())
    });
    super::theme::apply(&settings.theme, window, cx);
    window.refresh();
    settings.save(&dir)
}

/// Re-reads `history.json` into [`Ohg::history`].
pub fn reload_history(cx: &mut App) -> Task<()> {
    let call = sdk(cx).call::<Vec<HistoryEntry>>("historyList", ());
    cx.spawn(async move |cx| match call.await {
        Ok(list) => cx.update_global::<Ohg, _>(|o, cx| {
            o.history = list.into_iter().map(|e| (e.series_id.clone(), e)).collect();
            cx.refresh_windows();
        }),
        Err(err) => log::warn!("history: {err}"),
    })
}

#[derive(Debug, Clone)]
pub struct PlayRequest {
    pub series: SeriesSummary,
    pub episodes: Vec<Episode>,
    pub start: Episode,
    pub start_seconds: f64,
}

/// What a page asks of the shell.
#[derive(Debug, Clone)]
pub enum Nav {
    Open(SeriesSummary),
    Back,
    Play(PlayRequest),
    Toast { text: String, error: bool },
}

impl Nav {
    pub fn toast(text: impl Into<String>) -> Nav {
        Nav::Toast {
            text: text.into(),
            error: false,
        }
    }

    pub fn error(text: impl Into<String>) -> Nav {
        Nav::Toast {
            text: text.into(),
            error: true,
        }
    }
}
