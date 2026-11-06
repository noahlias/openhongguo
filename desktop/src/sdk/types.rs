//! Serde mirrors of the Go models (`sdk/model`, `docs/types.md`).
//! Returned models are snake_case; dispatcher wrappers and query options are camelCase.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Deserialize)]
pub struct Page<T> {
    #[serde(default = "Vec::new")]
    pub items: Vec<T>,
    #[serde(default)]
    pub cursor: String,
    #[serde(default)]
    pub has_more: bool,
}

#[derive(Debug, Clone, Default, Deserialize, PartialEq)]
#[serde(default)]
pub struct SeriesSummary {
    pub id: String,
    pub title: String,
    pub description: String,
    pub cover_url: String,
    pub episode_count: i32,
    pub author: String,
    pub category: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct InfoRow {
    pub kind: i32,
    pub text: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Series {
    pub id: String,
    pub title: String,
    pub description: String,
    pub cover_url: String,
    pub episode_count: i32,
    pub status: i32,
    pub pay_type: i32,
    pub content_type: i32,
    pub platform: i32,
    pub heat: i64,
    pub play_count: i64,
    pub follow_count: i64,
    pub digg_count: i64,
    pub right_text: String,
    pub is_multi_season: bool,
    pub season_index: i32,
    pub rows: Vec<InfoRow>,
}

impl Series {
    /// Platform 0 is a book or audiobook, which has no video to play.
    pub fn is_book(&self) -> bool {
        self.platform == 0
    }

    pub fn summary(&self) -> SeriesSummary {
        SeriesSummary {
            id: self.id.clone(),
            title: self.title.clone(),
            description: self.description.clone(),
            cover_url: self.cover_url.clone(),
            episode_count: self.episode_count,
            ..Default::default()
        }
    }
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct SeriesDetail {
    pub series: Option<Series>,
    pub episodes: Vec<Episode>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Episode {
    pub id: String,
    pub series_id: String,
    pub number: i32,
    pub title: String,
    pub playable: bool,
    pub vertical: bool,
    pub is_new: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Season {
    pub series_id: String,
    pub title: String,
    pub index: i32,
    pub episode_count: i32,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Album {
    pub id: String,
    pub title: String,
    pub seasons: Vec<Season>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Recommend {
    pub cell_title: String,
    pub items: Vec<SeriesSummary>,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Suggestion {
    pub text: String,
    pub keyword: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Stream {
    pub definition: String,
    pub quality: String,
    pub codec: String,
    pub width: i32,
    pub height: i32,
    pub bitrate: i64,
    pub url: String,
    pub backup_url: String,
    pub protected: bool,
}

impl Stream {
    fn height_rank(&self) -> i32 {
        if self.height > 0 {
            return self.height;
        }
        let digits: String = self
            .definition
            .chars()
            .take_while(char::is_ascii_digit)
            .collect();
        digits.parse().unwrap_or(self.width)
    }

    pub fn label(&self) -> String {
        if !self.definition.is_empty() {
            self.definition.clone()
        } else if self.height > 0 {
            format!("{}p", self.height)
        } else {
            self.quality.clone()
        }
    }
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(default)]
pub struct Playback {
    pub episode_id: String,
    pub duration_seconds: i32,
    pub expires_at: i64,
    pub protected: bool,
    pub streams: Vec<Stream>,
    pub audio_url: String,
}

impl Playback {
    /// Streams with a URL, lowest to highest. Port of `model.Playback.Select`'s ordering.
    pub fn usable(&self) -> Vec<Stream> {
        let mut v: Vec<Stream> = self
            .streams
            .iter()
            .filter(|s| !s.url.trim().is_empty())
            .cloned()
            .collect();
        v.sort_by_key(|s| (s.height_rank(), s.bitrate));
        v
    }

    /// "best"/"" picks the highest, "worst" the lowest, anything else matches a
    /// definition, quality label, or codec, falling back to the best.
    pub fn select(&self, quality: &str) -> Option<Stream> {
        let usable = self.usable();
        let want = quality.trim().to_lowercase();
        match want.as_str() {
            "" | "best" => return usable.last().cloned(),
            "worst" => return usable.first().cloned(),
            _ => {}
        }
        usable
            .iter()
            .find(|s| {
                [&s.definition, &s.quality, &s.codec]
                    .iter()
                    .any(|v| v.eq_ignore_ascii_case(&want))
            })
            .or(usable.last())
            .cloned()
    }

    /// True when the URLs expire within a minute (`playback.Expired`).
    pub fn expired(&self, now_unix: i64) -> bool {
        self.expires_at > 0 && self.expires_at < now_unix + 60
    }
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct ContentKey {
    pub key: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct PlaybackResult {
    pub playback: Playback,
    pub content_key: Option<ContentKey>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, PartialEq)]
#[serde(default)]
pub struct HistoryEntry {
    pub series_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub title: String,
    pub episode: i32,
    #[serde(skip_serializing_if = "is_zero")]
    pub episodes: i32,
    pub finished: bool,
    #[serde(skip_serializing_if = "is_zero_f")]
    pub position: f64,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub updated_at: String,
}

impl HistoryEntry {
    /// The whole series was watched: Continue hides these (`sec_continue.go:91`).
    pub fn completed(&self) -> bool {
        self.finished && self.episodes > 0 && self.episode >= self.episodes
    }
}

fn is_zero(v: &i32) -> bool {
    *v == 0
}

fn is_zero_f(v: &f64) -> bool {
    *v == 0.0
}

#[derive(Debug, Clone, Copy, Default, Deserialize)]
pub struct ResumePoint {
    pub episode: i32,
    pub start: i32,
}

#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DiscoverOptions {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub cursor: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub channel: String,
}

#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SearchOptions {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub cursor: String,
    pub page_size: i32,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub tab: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub filters: Vec<String>,
}

#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct EpisodeOptions {
    #[serde(skip_serializing_if = "String::is_empty")]
    pub cursor: String,
    pub page_size: i32,
}

#[cfg(test)]
mod tests {
    use super::*;

    fn stream(def: &str, h: i32, br: i64) -> Stream {
        Stream {
            definition: def.into(),
            height: h,
            bitrate: br,
            url: format!("u-{def}"),
            ..Default::default()
        }
    }

    #[test]
    fn select_orders_by_height_then_bitrate() {
        let pb = Playback {
            streams: vec![
                stream("720p", 1280, 2),
                stream("1080p", 1920, 1),
                stream("360p", 640, 9),
                Stream::default(),
            ],
            ..Default::default()
        };
        assert_eq!(pb.select("best").unwrap().definition, "1080p");
        assert_eq!(pb.select("").unwrap().definition, "1080p");
        assert_eq!(pb.select("worst").unwrap().definition, "360p");
        assert_eq!(pb.select("720P").unwrap().definition, "720p");
        assert_eq!(pb.select("4k").unwrap().definition, "1080p");
        assert_eq!(pb.usable().len(), 3);
    }

    #[test]
    fn completed_needs_the_last_episode() {
        let mut e = HistoryEntry {
            series_id: "s".into(),
            episode: 3,
            episodes: 10,
            finished: true,
            ..Default::default()
        };
        assert!(!e.completed());
        e.episode = 10;
        assert!(e.completed());
    }
}
