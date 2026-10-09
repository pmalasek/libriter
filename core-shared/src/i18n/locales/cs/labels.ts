export default {
  bookStatus: {
    finished: 'Doposlechnuto',
    started: 'Rozposlouchané',
    none: 'Neposlechnuto',
  },
  role: {
    admin: 'Administrátor',
    editor: 'Editor',
    reader: 'Čtenář',
  },
  auditAction: {
    user: {
      create: 'Založení účtu',
      role_change: 'Změna role',
      delete: 'Smazání účtu',
      password_reset: 'Reset hesla',
    },
    settings: {
      metadata_update: 'Změna zdrojů metadat',
      registration_update: 'Změna nastavení registrace',
      library_update: 'Změna nastavení knihovny',
    },
    book: {
      delete: 'Smazání knihy',
    },
    author: {
      delete: 'Smazání autora',
    },
    series: {
      delete: 'Smazání série',
    },
    scanner: {
      rescan: 'Spuštění kontroly knihovny',
    },
    library: {
      repair_apply: 'Oprava kapitol',
      merge_books: 'Sloučení rozdělených knih',
      import: 'Import knih',
    },
  },
  viewMode: {
    tiles: 'Dlaždice',
    small: 'Malé dlaždice',
    list: 'Seznam',
  },
  authorSort: {
    last_name: 'Příjmení',
    first_name: 'Křestní jméno',
    books: 'Počet knih',
  },
  bookSort: {
    title: 'Série a název',
    author: 'Autor',
    published: 'První vydání',
    added: 'Datum přidání',
  },
  colorScheme: {
    teal: 'Tyrkysová',
    blue: 'Modrá',
    violet: 'Fialová',
    green: 'Zelená',
  },
  themeMode: {
    light: 'Světlý',
    dark: 'Tmavý',
    system: 'Systém',
  },
  sessionKind: {
    book: 'Kniha',
    series: 'Série',
    list: 'Seznam',
  },
  sessionFallbackTitle: 'Poslech',
  loginHint: 'Nepovinné. 3–32 znaků: písmena bez diakritiky, číslice, tečka, podtržítko a pomlčka.',
}
