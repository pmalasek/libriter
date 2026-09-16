# libriter-frontend

Webové rozhraní Libriteru: React 19, TypeScript, Tailwind v4, shadcn/ui (Radix),
`react-router` a `@tanstack/react-query`. Uživatelské texty jsou v češtině.

## Vývoj

```bash
npm install
npm run dev          # http://localhost:5173
```

Backend musí běžet zvlášť (`make dev-backend` v korenu repozitáře). Vite proxuje
`/api` a `/health` na `http://localhost:8080`, takže CORS není potřeba. Jiný port
backendu: `BACKEND_URL=http://localhost:9000 npm run dev`.

Dev server naslouchá na `0.0.0.0`, takže je dostupný i z jiného počítače – použijte
adresu z řádku **Network** ve výpisu Vite.

## Build

```bash
npm run build
```

Výstup jde **do backendu** (`../libriter-backend/internal/web/dist`), odkud se
přes `go:embed` vkompiluje do binárky. Žádný krok s kopírováním není potřeba.
Adresář `dist` drží v gitu soubor `.gitkeep`, aby `go build` prošel i bez
sestaveného frontendu; po každém buildu ho Vite plugin obnoví.

## Struktura

| Cesta | Obsah |
|-------|-------|
| `src/api/` | Typy zrcadlící Go modely, `apiFetch` a react-query hooky |
| `src/auth/` | Session v `localStorage`, kontext přihlášení, ochrana rout |
| `src/components/ui/` | Komponenty shadcn/ui (generované, lze upravovat) |
| `src/components/layout/` | Sidebar, topbar, loga z `../_image/` pro světlý a tmavý režim, rámec přihlašovacích stránek, přepínač vzhledu, uživatelské menu |
| `src/theme/` | Světlý/tmavý/systémový režim a nezávislá barevná schémata (tyrkysová, modrá, fialová, zelená), uložená v uživatelském profilu (pro nepřihlášené jen v prohlížeči) |
| `src/index.css` | Barvy, fonty a zaoblení celé aplikace (Tailwind v4 tokeny) |
| `src/pages/` | Jedna komponenta na stránku |
| `src/lib/format.ts` | České formátování délky, datumů a skloňování |

## Poznámky k API

- Seznamy vracejí při prázdném výsledku JSON `null`, ne `[]` – proto `asList()`.
- Backend používá `DisallowUnknownFields`, takže se posílají jen dokumentovaná pole.
- Chyby přicházejí jako JSON `{"error": "…"}`, ale middleware posílá 401/403 jako
  `text/plain` – parser v `src/api/client.ts` zvládá obojí.
- Knihy nesou jen `author_id` a `series_id`; jména autorů a sérií se spojují na klientovi.
