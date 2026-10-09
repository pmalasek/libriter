/**
 * Klíče react-query jádra (profil, nastavení přihlášení). Produkty je
 * rozšiřují o vlastní – stejné klíče ve všech aplikacích drží zneplatňování
 * cache pohromadě.
 */
export const coreQueryKeys = {
  user: (id: string) => ['users', id] as const,
  authConfig: ['auth', 'config'] as const,
}
