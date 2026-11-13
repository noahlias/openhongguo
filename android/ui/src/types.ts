// Mirrors of the Go models (sdk/model, docs/types.md) the app reads, after desktop/src/sdk/types.rs.
// Returned models are snake_case; dispatcher wrappers and query options are camelCase.

export interface Page<T> {
  items: T[];
  cursor: string;
  has_more: boolean;
}

export interface SeriesSummary {
  id: string;
  title: string;
  description?: string;
  cover_url?: string;
  episode_count?: number;
  author?: string;
  category?: string;
}

export interface InfoRow {
  kind: number;
  text: string;
}

export interface Series {
  id: string;
  title: string;
  description: string;
  cover_url: string;
  episode_count: number;
  status: number;
  pay_type: number;
  content_type: number;
  platform: number;
  heat: number;
  play_count: number;
  follow_count: number;
  digg_count: number;
  right_text: string;
  is_multi_season: boolean;
  season_index: number;
  rows?: InfoRow[];
}

export interface SeriesDetail {
  series?: Series | null;
  episodes?: Episode[];
}

export interface Episode {
  id: string;
  series_id: string;
  number: number;
  title: string;
  playable: boolean;
  vertical: boolean;
  is_new: boolean;
}

export interface Season {
  series_id: string;
  title: string;
  index: number;
  episode_count: number;
}

export interface Album {
  id: string;
  title: string;
  seasons: Season[];
}

export interface Recommend {
  cell_title: string;
  items: SeriesSummary[];
}

export interface Suggestion {
  text: string;
  keyword: string;
}

export interface Stream {
  definition: string;
  quality: string;
  codec: string;
  width: number;
  height: number;
  bitrate: number;
  url: string;
  backup_url: string;
  protected: boolean;
}

export interface Playback {
  episode_id: string;
  duration_seconds: number;
  expires_at: number;
  protected: boolean;
  streams: Stream[] | null;
  audio_url: string;
}

export interface ContentKey {
  key: string;
}

export interface PlaybackResult {
  playback: Playback;
  contentKey?: ContentKey;
}

export interface HistoryEntry {
  series_id: string;
  title?: string;
  episode: number;
  episodes?: number;
  finished: boolean;
  position?: number;
  updated_at?: string;
}

export interface ResumePoint {
  episode: number;
  start: number;
}

export interface DiscoverOptions {
  cursor?: string;
  channel?: string;
}

export interface SearchOptions {
  cursor?: string;
  pageSize: number;
  tab?: string;
  filters?: string[];
}

export interface EpisodeOptions {
  cursor?: string;
  pageSize: number;
}

/** Platform 0 is a book or audiobook, which has no video to play. */
export const isBook = (s: Series) => s.platform === 0;

export const summaryOf = (s: Series): SeriesSummary => ({
  id: s.id,
  title: s.title,
  description: s.description,
  cover_url: s.cover_url,
  episode_count: s.episode_count,
});

/** The whole series was watched: Continue hides these (sec_continue.go:91). */
export const completed = (e: HistoryEntry) => e.finished && (e.episodes ?? 0) > 0 && e.episode >= (e.episodes ?? 0);
