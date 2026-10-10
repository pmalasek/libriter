// Typy API Sceniteru. Zrcadlí JSON modely server/internal/sceniter/model
// a odpovědi handlerů v server/internal/sceniter/handler.

export type MatchState = 'unmatched' | 'auto' | 'manual' | 'ignored'
export type TitleKind = 'movie' | 'show'

/** Knihovna – podsložka VIDEO_ROOT s typem obsahu a zdrojem metadat. */
export interface Library {
  id: string
  dir: string
  name: string
  kind: TitleKind
  /** Jazyk metadat (ISO 639-1). */
  language: string
  /** Výchozí zdroj metadat (jméno pluginu). */
  provider?: string
  /** Ikona v menu (název z nabídky webu); chybí = podle typu. */
  icon?: string
  position: number
  enabled: boolean
  created_at: string
  updated_at: string
  /** Filmy nebo seriály se soubory. */
  item_count: number
}

/** POST/PATCH /sceniter/admin/libraries – dir a kind jen při založení. */
export interface LibraryRequest {
  dir?: string
  name?: string
  kind?: TitleKind
  language?: string
  /** '' = bez automatického párování. */
  provider?: string
  /** '' = výchozí ikona podle typu. */
  icon?: string
  enabled?: boolean
}

/** Zdroj video metadat (plugin). */
export interface MetadataProvider {
  name: string
  title: string
  kinds: TitleKind[]
  languages: string[]
}

export type CreditRole = 'cast' | 'director' | 'writer' | 'creator'

/** Osoba z obsazení nebo štábu titulu. */
export interface Credit {
  person_id: string
  name: string
  role: CreditRole
  character?: string
  /** Fotka osoby (název v úložišti obrázků). */
  profile_path?: string
  updated_at?: string
}

/** Osoba (herec, režisér, scenárista, tvůrce). */
export interface Person {
  id: string
  name: string
  profile_path?: string
  updated_at: string
}

/** Položka filmografie: film nebo seriál z knihovny a role v něm. */
export interface PersonCredit {
  roles: CreditRole[]
  character?: string
  movie?: Movie
  show?: Show
}

export interface PersonDetail extends Person {
  credits: PersonCredit[]
}

export interface Movie {
  id: string
  library_id: string
  title: string
  original_title?: string
  year?: number
  overview?: string
  tagline?: string
  runtime_min?: number
  /** Zdroj (plugin) a ID titulu u něj; chybí u nespárovaného titulu. */
  meta_provider?: string
  meta_id?: string
  /** ID u ostatních databází: { imdb: 'tt…', tmdb: '603', csfd: '9499' }. */
  external_ids: Record<string, string>
  poster_path?: string
  backdrop_path?: string
  /** Hodnocení zdroje v procentech (0–100). */
  rating?: number
  /** Obsazení a štáb – jen v detailu, ve výpisech prázdné. */
  credits: Credit[]
  genres: string[]
  match_state: MatchState
  /** Kdy se automatické párování naposledy nepovedlo (žádný zdroj nenašel jistou shodu). */
  match_attempted_at?: string
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
  library_id: string
  title: string
  original_title?: string
  year?: number
  overview?: string
  status?: string
  meta_provider?: string
  meta_id?: string
  external_ids: Record<string, string>
  poster_path?: string
  backdrop_path?: string
  rating?: number
  /** Obsazení a štáb – jen v detailu, ve výpisech prázdné. */
  credits: Credit[]
  genres: string[]
  match_state: MatchState
  /** Kdy se automatické párování naposledy nepovedlo (žádný zdroj nenašel jistou shodu). */
  match_attempted_at?: string
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
  /** Pozadí seriálu – náhrada za chybějící obrázek dílu. */
  show_backdrop_path?: string
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

/** Řádek domovské stránky za jednu knihovnu (nově přidané). */
export interface HomeLibrary {
  library: Library
  movies: Movie[]
  episodes: Episode[]
}

export interface Home {
  continue: ContinueItem[]
  libraries: HomeLibrary[]
  watchlist: WatchlistItem[]
}

export interface SearchResults {
  movies: Movie[]
  shows: Show[]
  people: Person[]
}

/** Průběh obnovy metadat spárovaných titulů (administrace). */
export interface RefreshStatus {
  running: boolean
  done: number
  total: number
  failed: number
}

/** Průběh automatického párování nespárovaných titulů (administrace). */
export interface MatchStatus {
  running: boolean
  done: number
  total: number
  matched: number
  failed: number
  /** Odložené – zdroj odmítal požadavky, dopárují se po pauze. */
  deferred: number
  /** Titul, který se právě páruje. */
  current?: string
  /** Konec pauzy zdroje, po kterém párování pokračuje. */
  paused_until?: string
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
  metadata_providers: MetadataProvider[]
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
    /** Nespárované tituly, pro které žádný zdroj nenašel jistou shodu. */
    failed_movies: number
    failed_shows: number
  }
  scanner: SceniterScannerStatus
  transcodes: number
  /** Je nainstalovaný aspoň jeden zdroj video metadat. */
  metadata: boolean
}

/** Výsledek hledání u zdroje metadat (pro ruční párování). */
export interface MetadataResult {
  id: string
  url?: string
  title: string
  original_title?: string
  year?: number
  note?: string
  image_url?: string
}

export interface MetadataSearchResponse {
  provider: string
  results: MetadataResult[]
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
