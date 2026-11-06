use std::borrow::Cow;

use gpui_kit::assets::{Assets, IconName};
use gpui_kit::component::Icon;
use gpui_kit::*;

gpui_kit::assets::icon_assets!(
    ExtraIcons,
    [
        RotateCcwClock,
        House,
        Compass,
        Volume2,
        Volume1,
        VolumeX,
        SkipForward,
        SkipBack,
        Lock,
        ListVideo,
        Clapperboard,
        Flame,
        Repeat
    ]
);

/// The default icon set plus the few above the app needs.
pub struct AppAssets;

impl AssetSource for AppAssets {
    fn load(&self, path: &str) -> Result<Option<Cow<'static, [u8]>>> {
        if let Some(bytes) = ExtraIcons.load(path)? {
            return Ok(Some(bytes));
        }
        Assets.load(path)
    }

    fn list(&self, path: &str) -> Result<Vec<SharedString>> {
        let mut paths = Assets.list(path)?;
        paths.extend(ExtraIcons.list(path)?);
        paths.sort();
        paths.dedup();
        Ok(paths)
    }
}

pub fn icon(name: IconName, size: f32, color: Hsla) -> Icon {
    Icon::from(name).size(px(size)).text_color(color)
}
