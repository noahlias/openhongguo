//! `desktop.json` in the shared config dir. `anonymous.json` and
//! `history.json` beside it belong to the SDK and are shared with the CLI/TUI.

use std::path::{Path, PathBuf};

use anyhow::{Context as _, Result};
use serde::{Deserialize, Serialize};

const VERSION: u32 = 1;

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct Settings {
    pub version: u32,
    /// "zh" or "en".
    pub language: String,
    /// "auto", "dark" or "light".
    pub theme: String,
    /// "best", "worst" or a definition such as "720p".
    pub quality: String,
    pub autoplay: bool,
    pub volume: f64,
    pub window: WindowSize,
    /// Shows the debug panel (F12).
    pub debug: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
pub struct WindowSize {
    pub width: f32,
    pub height: f32,
}

impl Default for Settings {
    fn default() -> Settings {
        Settings {
            version: VERSION,
            language: "zh".into(),
            theme: "dark".into(),
            quality: "best".into(),
            autoplay: true,
            volume: 80.0,
            window: WindowSize {
                width: 1280.0,
                height: 820.0,
            },
            debug: false,
        }
    }
}

/// `$OHG_CONFIG_DIR`, else `~/.config/openhongguo` (as `cmd/ohg`).
pub fn config_dir() -> Result<PathBuf> {
    if let Some(dir) = std::env::var_os("OHG_CONFIG_DIR").filter(|v| !v.is_empty()) {
        return Ok(PathBuf::from(dir));
    }
    Ok(dirs::config_dir()
        .context("no config directory")?
        .join("openhongguo"))
}

impl Settings {
    pub fn path(dir: &Path) -> PathBuf {
        dir.join("desktop.json")
    }

    /// A missing file gives the defaults; a malformed one is an error.
    pub fn load(dir: &Path) -> Result<Settings> {
        let path = Settings::path(dir);
        match std::fs::read(&path) {
            Ok(data) => {
                serde_json::from_slice(&data).with_context(|| format!("reading {}", path.display()))
            }
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(Settings::default()),
            Err(e) => Err(e).with_context(|| format!("reading {}", path.display())),
        }
    }

    pub fn save(&self, dir: &Path) -> Result<()> {
        std::fs::create_dir_all(dir)?;
        let path = Settings::path(dir);
        let tmp = path.with_extension("json.tmp");
        let mut data = serde_json::to_vec_pretty(self)?;
        data.push(b'\n');
        std::fs::write(&tmp, data)?;
        std::fs::rename(&tmp, &path).with_context(|| format!("writing {}", path.display()))
    }

    pub fn zh(&self) -> bool {
        self.language != "en"
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trips_and_defaults_missing_fields() {
        let dir = std::env::temp_dir().join(format!("ohg-settings-{}", std::process::id()));
        assert_eq!(Settings::load(&dir).unwrap(), Settings::default());
        let s = Settings {
            language: "en".into(),
            volume: 55.0,
            ..Default::default()
        };
        s.save(&dir).unwrap();
        assert_eq!(Settings::load(&dir).unwrap(), s);
        std::fs::write(Settings::path(&dir), r#"{"autoplay":false}"#).unwrap();
        let partial = Settings::load(&dir).unwrap();
        assert!(!partial.autoplay);
        assert_eq!(partial.quality, "best");
        std::fs::remove_dir_all(&dir).unwrap();
    }
}
