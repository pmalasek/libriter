import { t, type UILanguage } from './i18n'

// Typy jádra – účty, přihlášení, tokeny a společná administrace. Zrcadlí
// JSON modely jádra serveru (server/internal/core/model/model.go). Pole
// s `omitempty` na Go straně jsou zde volitelná.

export type Role = 'admin' | 'editor' | 'reader'

export type ColorScheme = 'teal' | 'blue' | 'violet' | 'green'

export type ThemeMode = 'light' | 'dark' | 'system'

export interface UserAppearance {
  color_scheme: ColorScheme
  theme_mode: ThemeMode
}

export interface User extends UserAppearance {
  id: string
  /** Jazyk rozhraní z profilu. */
  ui_language: UILanguage
  display_name: string
  email: string
  /** Volitelné přihlašovací jméno; prázdný řetězec = uživatel login nemá. */
  login: string
  role: Role
  created_at: string
  updated_at: string
}

/**
 * Formát přihlašovacího jména – musí odpovídat backendu
 * (internal/service/login.go). Hodí se do atributu `pattern` u inputu.
 */
export const LOGIN_PATTERN = '[A-Za-z0-9._\\-]{3,32}'

export function loginHint(): string {
  return t('labels.loginHint')
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

/**
 * POST /auth/mobile-token – dlouhodobý token mobilní aplikace (výchozí rok).
 * Otevírá API stejně jako přihlašovací token, ale ne audio; další mobilní
 * token jím vyrazit nejde, obnovuje se skutečným přihlášením.
 */
export interface MobileTokenRequest {
  /** Jen do serverového logu; nikde se neukládá. */
  device_name?: string
}

export type MobileToken = StreamToken

export interface AuthResponse {
  user: User
  token: string
}

// Backend používá DisallowUnknownFields – posílat jen tato pole, nic navíc.
export interface LoginRequest {
  /** Porovnává se s loginem i s e-mailem uživatele. */
  login: string
  password: string
}

export interface RegisterRequest {
  display_name: string
  email: string
  login?: string
  password: string
  /** Jazyk rozhraní, ve kterém se uživatel registroval. */
  ui_language?: UILanguage
}

/** Prázdný login uživateli login odebere. */
export interface UpdateUserRequest {
  display_name: string
  email: string
  login: string
}

export interface ChangePasswordRequest {
  password: string
}

export function roleLabel(role: Role): string {
  return t(`labels.role.${role}`)
}

/** GET /auth/config – veřejné, přihlašovací stránka podle toho skrývá registraci. */
export interface AuthConfig {
  registration_enabled: boolean
  default_role: Role
}

/** POST /admin/users – zakládá účet s libovolnou rolí. */
export interface CreateUserRequest {
  display_name: string
  email: string
  login?: string
  password: string
  role: Role
}

/** PUT /users/{id}/role */
export interface SetRoleRequest {
  role: Role
}

/** GET a PUT /admin/settings/registration */
export interface RegistrationSettings {
  enabled: boolean
  default_role: Extract<Role, 'reader' | 'editor'>
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

const AUDIT_ACTIONS = [
  'user.create',
  'user.role_change',
  'user.delete',
  'user.password_reset',
  'settings.metadata_update',
  'settings.registration_update',
  'settings.library_update',
  'settings.plugin_update',
  'plugins.reload',
  'book.delete',
  'author.delete',
  'series.delete',
  'scanner.rescan',
  'library.repair_apply',
  'library.merge_books',
] as const

/** Čitelný název akce v auditu; neznámá akce se ukáže, jak přišla. */
export function auditActionLabel(action: string): string {
  return (AUDIT_ACTIONS as readonly string[]).includes(action)
    ? t(`labels.auditAction.${action as (typeof AUDIT_ACTIONS)[number]}`)
    : action
}

// --- pluginy metadat (GET /admin/plugins) ---

export type PluginKind = 'book' | 'author' | 'movie' | 'show'

/** Položka konfigurace pluginu; tajná hodnota se nevrací, jen is_set. */
export interface PluginConfigField {
  key: string
  label: string
  secret?: boolean
  required?: boolean
  env?: string
  value?: string
  is_set: boolean
  from_env: boolean
}

export interface PluginInfo {
  name: string
  title: string
  version: string
  kinds: PluginKind[]
  languages: string[]
  path: string
  config: PluginConfigField[]
}

export interface PluginsResponse {
  dir: string
  plugins: PluginInfo[]
}
