// Typy zrcadlí JSON modely backendu (libriter-backend/internal/model/model.go).
// Pole s `omitempty` na Go straně jsou zde volitelná.

export type Role = 'admin' | 'editor' | 'reader'
export type ColorScheme = 'teal' | 'blue' | 'violet' | 'green'
export type ThemeMode = 'light' | 'dark' | 'system'

export interface UserAppearance {
  color_scheme: ColorScheme
  theme_mode: ThemeMode
}

export interface User extends UserAppearance {
  id: string
  display_name: string
  email: string
  role: Role
  created_at: string
  updated_at: string
}

export interface Author {
  id: string
  first_name: string
  middle_name: string
  last_name: string
  /** Celé jméno složené z částí – dopočítává backend. */
  name: string
  bio?: string
  /** Název souboru v AUTHOR_IMAGE_ROOT; obrázek se čte z /authors/{id}/image. */
  image_path?: string
  birth_year?: number
  death_year?: number
  created_at: string
}

export interface Series {
  id: string
  title: string
  description?: string
  created_at: string
}

export interface Book {
  id: string
  /** Kniha může mít víc autorů; pořadí určuje backend (hlavní autor první). */
  authors: Author[]
  series_id?: string
  series_position?: number
  title: string
  narrator?: string
  duration_seconds: number
  /** Počet kapitol (audio souborů) knihy – odvozuje ho backend. */
  chapter_count: number
  /** Adresář s audio soubory, relativně ke kořeni knihovny (AUDIO_ROOT). */
  file_path: string
  cover_path?: string
  language: string
  description?: string
  internal_rating?: number
  /** Rok (prvního) vydání – přebírá se ze zdrojů metadat, nemusí být známý. */
  published_year?: number
  created_at: string
  updated_at: string
}

/** Kapitola knihy = jeden audio soubor (GET /books/{id}/chapters). */
export interface Chapter {
  id: string
  /** Pořadí v knize; po ručním seřazení jde o řadu 1..N. */
  position: number
  title: string
  /** Název souboru bez adresáře – podle něj se pozná správné pořadí. */
  file_name: string
  /** Začátek kapitoly v rámci celé knihy (součet délek předchozích). */
  start_offset_seconds: number
  duration_seconds: number
}

/** PUT /books/{id}/chapters/order – všechny kapitoly knihy v novém pořadí. */
export interface ReorderChaptersRequest {
  chapter_ids: string[]
}

/** Z čeho poslech vznikl: jedna kniha, celá série, nebo vlastní seznam. */
export type PlaySessionKind = 'book' | 'series' | 'list'

/**
 * Poslech (session) – to, co se právě přehrává. Rozposlouchaných může být víc
 * naráz; pozice se drží na serveru, aby šlo pokračovat na jiném zařízení.
 */
export interface PlaySession {
  id: string
  kind: PlaySessionKind
  /** Kniha nebo série, ze které poslech vznikl; u seznamu chybí. */
  source_id?: string
  /** Název nese jen seznam – u knihy a série se bere z knihovny. */
  title?: string
  current_book_id?: string
  playback_speed: number
  /** Vyplněné, když se poslech dostal na konec poslední kapitoly. */
  finished_at?: string
  created_at: string
  updated_at: string
  items: PlaySessionItem[]
}

/** Jedna kniha poslechu i s vlastní rozposlouchanou pozicí. */
export interface PlaySessionItem {
  book_id: string
  position: number
  /** Chybí, když kapitola zmizela při opravě knihovny – začne se od začátku. */
  chapter_id?: string
  /** Pozice v kapitole, ne v celé knize. */
  position_seconds: number
}

/** POST /sessions – založení nebo pokračování poslechu. */
export type CreateSessionRequest =
  | { kind: 'book'; book_id: string }
  | { kind: 'series'; series_id: string }
  | { kind: 'list'; title?: string; book_ids?: string[]; series_ids?: string[] }

/** PUT /sessions/{id}/position */
export interface SessionPositionRequest {
  book_id: string
  chapter_id?: string
  position_seconds: number
  playback_speed: number
  /** Doposlechnuto do konce; další změna pozice příznak zase zruší. */
  finished?: boolean
}

/** POST /sessions/{id}/items – přidání knih a sérií na konec poslechu. */
export interface SessionItemsRequest {
  book_ids?: string[]
  series_ids?: string[]
}

/**
 * GET /auth/stream-token – krátkodobý token pro adresu audio souboru.
 * Prvek <audio> neumí poslat hlavičku Authorization, přihlašovací token ale
 * do adresy nepatří.
 */
export interface StreamToken {
  token: string
  expires_at: string
}

export interface AuthResponse {
  user: User
  token: string
}

// Backend používá DisallowUnknownFields – posílat jen tato pole, nic navíc.
export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  display_name: string
  email: string
  password: string
}

export interface UpdateUserRequest {
  display_name: string
  email: string
}

export interface ChangePasswordRequest {
  password: string
}

/**
 * PUT /authors/{id} – úplná náhrada, takže se posílají i pole, která formulář
 * neukazuje (image_path), jinak by je server vymazal.
 */
export interface AuthorRequest {
  first_name: string
  middle_name: string
  last_name: string
  bio: string | null
  image_path: string | null
  birth_year: number | null
  death_year: number | null
}

/**
 * PATCH /books/{id} – posílat JEN pole, která se opravdu mění. Vynechané pole
 * zůstane beze změny, `null` sloupec vyprázdní. `file_path` server nepřijímá
 * (DisallowUnknownFields), cestu k audiu spravuje výhradně scanner.
 */
export interface BookPatchRequest {
  author_ids?: string[]
  series_id?: string | null
  series_position?: number | null
  title?: string
  narrator?: string | null
  duration_seconds?: number
  language?: string
  description?: string | null
  internal_rating?: number | null
  published_year?: number | null
}

/** POST /series */
export interface SeriesRequest {
  title: string
  description: string | null
}

/**
 * Výsledek GET /metadata/search?q=
 *
 * Zdroje se zkoušejí v pořadí z METADATA_PROVIDERS a vrátí se výsledky
 * prvního, který něco najde – `source` říká, který to byl. `author` a `year`
 * nemusí být vyplněné, ne každý zdroj je dává už v seznamu.
 */
export interface MetadataSearchResult {
  id: number
  title: string
  author: string
  year: number
  url: string
  source: string
}

/** Jeden autor z metadat knihy – jméno je rozdělené stejně jako u autorů knihovny. */
export interface BookMetadataAuthor {
  name: string
  first_name: string
  middle_name: string
  last_name: string
}

/** Odpověď GET /metadata/book?url= – nevyplněná pole zůstávají nulová. */
export interface BookMetadata {
  id: number
  title: string
  /** Autor (nebo autoři oddělení čárkou) tak, jak ho píše zdroj. */
  author: string
  author_id: number
  /** Totéž rozebrané na jednotlivé autory; prázdné, když zdroj autora neuvádí. */
  authors: BookMetadataAuthor[] | null
  description: string
  genres: string[] | null
  cover_url: string
  /** Hodnocení zdroje v procentech (0–100) – není totéž co internal_rating (1–5). */
  rating: number
  publisher: string
  /** Rok prvního vydání díla (u překladů rok originálu), pokud ho zdroj zná. */
  year: number
  /** Název originálu u překladů; jinak prázdný. */
  original_title: string
  /** Název knižní série; prázdný u knihy mimo sérii i u zdrojů, které série neznají. */
  series: string
  /** Pořadí dílu v sérii; 0, když ho zdroj neuvádí. */
  series_position: number
  source_url: string
  source: string
}

/** Výsledek GET /metadata/author/search?q= */
export interface AuthorSearchResult {
  id: number
  name: string
  /** Roky života nebo nejznámější dílo – odliší jmenovce. */
  note: string
  birth_year: number
  death_year: number
  url: string
  source: string
}

/** Odpověď GET /metadata/author?url= */
export interface AuthorMetadata {
  id: number
  name: string
  /** Jméno rozdělené stejně, jako se ukládá u autora – dopočítává backend. */
  first_name: string
  middle_name: string
  last_name: string
  bio: string
  /**
   * Jména, pod kterými autor vydává, rozdělená stejně jako `name`. Zdroj vede
   * autora pod občanským jménem (Frode Sander Øien), ale knihy jsou podepsané
   * pseudonymem (Samuel Bjørk) – ten se do knihovny ukládá.
   */
  pseudonyms: BookMetadataAuthor[] | null
  /** Adresa fotky u zdroje; stahuje se až přes PUT /authors/{id}/image. */
  image_url: string
  birth_year: number
  death_year: number
  source_url: string
  source: string
}

/** Zdroje metadat podle toho, co umí (GET /metadata/sources). */
export interface MetadataSources {
  books: string[]
  authors: string[]
}

/** Čitelné názvy zdrojů metadat (hodnoty pole `source`). */
export const METADATA_SOURCE_LABELS: Record<string, string> = {
  databazeknih: 'databazeknih.cz',
  cbdb: 'cbdb.cz',
  openlibrary: 'OpenLibrary',
  googlebooks: 'Google Books',
}

export const ROLE_LABELS: Record<Role, string> = {
  admin: 'Administrátor',
  editor: 'Editor',
  reader: 'Čtenář',
}

// --- administrace ---

/** GET /auth/config – veřejné, přihlašovací stránka podle toho skrývá registraci. */
export interface AuthConfig {
  registration_enabled: boolean
  default_role: Role
}

/** POST /admin/users – zakládá účet s libovolnou rolí. */
export interface CreateUserRequest {
  display_name: string
  email: string
  password: string
  role: Role
}

/** PUT /users/{id}/role */
export interface SetRoleRequest {
  role: Role
}

/** Zdroj metadat v nastavení; pořadí v poli je pořadí, ve kterém se zkouší. */
export interface AdminProvider {
  name: string
  enabled: boolean
  supports_authors: boolean
  supports_images: boolean
}

/** GET /admin/settings/metadata */
export interface MetadataSettings {
  providers: AdminProvider[]
  google_books_api_key: string
}

/** PUT /admin/settings/metadata – backend má DisallowUnknownFields. */
export interface MetadataSettingsRequest {
  providers: { name: string; enabled: boolean }[]
  google_books_api_key: string
}

/** GET a PUT /admin/settings/registration */
export interface RegistrationSettings {
  enabled: boolean
  default_role: Extract<Role, 'reader' | 'editor'>
}

/** GET /admin/scanner */
export interface ScannerStatus {
  running: boolean
  /** „startup“ při startu serveru, „manual“ z administrace. */
  trigger: string
  started_at: string | null
  finished_at: string | null
  processed: number
  ingested: number
  errors: number
  last_error: string
  watcher_active: boolean
}

export interface RepairBook {
  id: string
  title: string
  /** Adresář knihy – jinde v API se cesty k audiu nevracejí. */
  file_path: string
}

/** GET /admin/library/repair – náhled, nic nemění. */
export interface RepairPlan {
  rescan: RepairBook[]
  duplicates: RepairBook[]
}

/** POST /admin/library/repair */
export interface RepairResult {
  plan: RepairPlan
  deleted_chapters: number
  deleted_books: number
}

/** Kniha v plánu sloučení; cesta a album tag jinde v API nejsou. */
export interface MergeBook {
  id: string
  title: string
  file_path: string
  /** Album tag ze souborů; prázdný u knih z dřívějších scanů. */
  album_tag: string
  chapter_count: number
  created_at: string
}

/** Jedna rozdělená kniha: zdroje se slijí do cíle. */
export interface MergeGroup {
  target: MergeBook
  sources: MergeBook[]
}

/** GET /admin/library/merge – náhled, nic nemění. */
export interface MergePlan {
  groups: MergeGroup[]
}

/** POST /admin/library/merge */
export interface MergeResult {
  plan: MergePlan
  merged_books: number
  moved_chapters: number
}

/** GET /admin/stats */
export interface LibraryStats {
  books: number
  authors: number
  series: number
  users: number
  chapters: number
  total_duration_seconds: number
  books_without_cover: number
  books_without_description: number
  /** Kapitoly s délkou 1 s – vzniknou, když na serveru chybí ffprobe. */
  placeholder_chapters: number
  books_with_placeholder_chapters: number
}

export interface DiskUsage {
  path: string
  free_bytes: number
  total_bytes: number
}

/** GET /admin/system */
export interface SystemInfo {
  version: string
  go_version: string
  os: string
  env: string
  started_at: string
  uptime_seconds: number
  db_path: string
  audio_root: string
  cover_root: string
  author_image_root: string
  ffprobe_available: boolean
  ffprobe_path: string
  /** null na systémech, kde volné místo nejde zjistit. */
  disk: DiskUsage | null
}

/** Záznam v audit logu (GET /admin/audit). */
export interface AuditEntry {
  id: number
  actor_id?: string
  /** E-mail v době akce; zůstává i po smazání účtu. */
  actor_email: string
  action: string
  target_type: string
  target_id: string
  target_label: string
  details: Record<string, unknown>
  created_at: string
}

/** Stránka auditu; next_before je kurzor pro další stránku. */
export interface AuditPage {
  items: AuditEntry[]
  next_before: number | null
}

/** Čitelné názvy akcí v auditu. */
export const AUDIT_ACTION_LABELS: Record<string, string> = {
  'user.create': 'Založení účtu',
  'user.role_change': 'Změna role',
  'user.delete': 'Smazání účtu',
  'user.password_reset': 'Reset hesla',
  'settings.metadata_update': 'Změna zdrojů metadat',
  'settings.registration_update': 'Změna nastavení registrace',
  'book.delete': 'Smazání knihy',
  'author.delete': 'Smazání autora',
  'series.delete': 'Smazání série',
  'scanner.rescan': 'Spuštění kontroly knihovny',
  'library.repair_apply': 'Oprava kapitol',
  'library.merge_books': 'Sloučení rozdělených knih',
}
