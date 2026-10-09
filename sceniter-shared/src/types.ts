// Typy API Sceniteru. Zrcadlí JSON modely server/internal/sceniter/model
// a odpovědi handlerů v server/internal/sceniter/handler.

export type MatchState = 'unmatched' | 'auto' | 'manual' | 'ignored'
export type TitleKind = 'movie' | 'show'

export interface Credit {
  name: string
  role: 'cast' | 'director' | 'writer' | 'creator'
  character?: string
}

export interface Movie {
  id: string
  title: string
  original_title?: string
  year?: number
  overview?: string
  tagline?: string
  runtime_min?: number
  tmdb_id?: number
  imdb_id?: string
  poster_path?: string
  backdrop_path?: string
  tmdb_rating?: number
  credits: Credit[]
  genres: string[]
  match_state: MatchState
  source_key: string
  created_at: string
  updated_at: string
  /** Délka hlavního souboru (s). */
  duration_sec?: number
  /** Hlavní soubor filmu – ten se přehrává. */
  media_id?: string
}

export interface Show {
  id: string
  title: string
  original_title?: string
  year?: number
  overview?: string
  status?: string
  tmdb_id?: number
  poster_path?: string
  backdrop_path?: string
  tmdb_rating?: number
  credits: Credit[]
  genres: string[]
  match_state: MatchState
  source_key: string
  created_at: string
  updated_at: string
  season_count: number
  episode_count: number
}

export interface Episode {
  id: string
  season_id: string
  show_id: string
  season: number
  number: number
  title?: string
  overview?: string
  air_date?: string
  still_path?: string
  runtime_min?: number
  created_at: string
  duration_sec?: number
  media_id?: string
  show_title?: string
}

export interface WatchProgress {
  movie_id?: string
  episode_id?: string
  media_id?: string
  position_sec: number
  duration_sec?: number
  completed: boolean
  recorded_at: string
  updated_at: string
}

export interface EpisodeWithProgress extends Episode {
  progress?: WatchProgress
}

export interface Season {
  id: string
  show_id: string
  number: number
  title?: string
  overview?: string
  poster_path?: string
  episodes: EpisodeWithProgress[]
}

export interface Stream {
  index: number
  kind: 'video' | 'audio' | 'subtitle'
  codec?: string
  language?: string
  title?: string
  channels?: number
  is_default: boolean
  is_forced: boolean
}

export interface ExternalSubtitle {
  id: string
  language?: string
  format: string
  is_forced: boolean
}

export interface MediaFile {
  id: string
  movie_id?: string
  episode_id?: string
  file_path: string
  size_bytes: number
  container?: string
  duration_sec?: number
  bitrate?: number
  video_codec?: string
  video_profile?: string
  width?: number
  height?: number
  pix_fmt?: string
  probed_at?: string
  probe_error?: string
  streams: Stream[]
  external_subtitles: ExternalSubtitle[]
}

export interface UserState {
  in_watchlist: boolean
  rating?: number
  progress?: WatchProgress
}

export interface MovieDetail extends Movie {
  files: MediaFile[]
  user: UserState
}

export interface ShowDetail extends Show {
  seasons: Season[]
  user: UserState
  up_next?: EpisodeWithProgress
}

export interface EpisodeDetail {
  episode: Episode
  show: Show
  files: MediaFile[]
  progress?: WatchProgress | null
  next?: Episode | null
}

export interface ContinueItem {
  kind: 'movie' | 'episode'
  movie?: Movie
  episode?: Episode
  show?: Show
  progress?: WatchProgress
  /** Další díl, který uživatel ještě nezačal. */
  next: boolean
}

export interface WatchlistItem {
  kind: TitleKind
  movie?: Movie
  show?: Show
  added_at: string
}

export interface Home {
  continue: ContinueItem[]
  recent_movies: Movie[]
  recent_episodes: Episode[]
  watchlist: WatchlistItem[]
}

export interface SearchResults {
  movies: Movie[]
  shows: Show[]
}

export type PlaybackMode = 'direct' | 'remux' | 'transcode_audio' | 'transcode'

export interface SubtitleTrack {
  key: string
  url: string
  language?: string
  title?: string
  forced: boolean
  external: boolean
  default: boolean
}

/** GET /sceniter/media/{id}/playback – adresy už nesou token (?t=). */
export interface Playback {
  media_id: string
  url: string
  hls: boolean
  duration_sec?: number
  token_expires_at: string
  mode: PlaybackMode
  reasons: string[]
  audio_index?: number
  audio: Stream[]
  subtitles: SubtitleTrack[]
  unsupported_subtitles: number[]
  transcoding_available: boolean
}

/** PUT /sceniter/progress */
export interface ProgressRequest {
  movie_id?: string
  episode_id?: string
  media_id?: string
  position_sec: number
  duration_sec?: number
  watched_seconds?: number
  completed?: boolean
  device_id?: string
  recorded_at?: string
}

export interface SceniterConfig {
  enabled: boolean
  tmdb_enabled: boolean
  transcoding: boolean
  profiles: string[]
}

// --- administrace ---

export interface SceniterScannerStatus {
  running: boolean
  trigger: string
  started_at: string | null
  finished_at: string | null
  processed: number
  added: number
  removed: number
  errors: number
  last_error: string
  watcher_active: boolean
  ffprobe: boolean
}

export interface SceniterStats {
  library: {
    movies: number
    shows: number
    episodes: number
    files: number
    total_bytes: number
    unmatched_movies: number
    unmatched_shows: number
  }
  scanner: SceniterScannerStatus
  transcodes: number
  tmdb: boolean
}

export interface TmdbResult {
  id: number
  title: string
  original_title: string
  year?: number
  overview: string
  poster_path?: string
  popularity: number
}

export interface TranscodeSession {
  id: string
  user_id: string
  media_id: string
  mode: PlaybackMode
  started_at: string
  last_access: string
  running: boolean
  error?: string
}

export interface TitlePatch {
  title?: string
  original_title?: string
  year?: number
  overview?: string
}
