/**
 * Klíče react-query jádra (profil, nastavení přihlášení). Produkty je
 * rozšiřují o vlastní – stejné klíče ve všech aplikacích drží zneplatňování
 * cache pohromadě.
 */
export const coreQueryKeys = {
  user: (id: string) => ['users', id] as const,
  authConfig: ['auth', 'config'] as const,
  version: ['version'] as const,
  adminPlugins: ['admin', 'plugins'] as const,

  // ---- správa uživatelů, registrace a audit (sdílená administrace) ----
  // Hodnoty se nesmí změnit: Libriter je má i v adminKeys a jeho mutace
  // přes ně zestarávají audit a seznam uživatelů.
  adminUsers: ['admin', 'users'] as const,
  adminRegistration: ['admin', 'settings', 'registration'] as const,
  adminAudit: ['admin', 'audit'] as const,

  // --- přihlášená zařízení (profil) ---
  userDevices: (id: string) => ['users', id, 'devices'] as const,
}
