export default {
  bookStatus: {
    finished: 'Abgeschlossen',
    started: 'Begonnen',
    none: 'Nicht angehört',
  },
  role: {
    admin: 'Administrator',
    editor: 'Bearbeiter',
    reader: 'Leser',
  },
  auditAction: {
    user: {
      create: 'Konto erstellt',
      role_change: 'Rolle geändert',
      delete: 'Konto gelöscht',
      password_reset: 'Passwort zurückgesetzt',
    },
    settings: {
      metadata_update: 'Metadatenquellen geändert',
      registration_update: 'Registrierungseinstellungen geändert',
      library_update: 'Bibliothekseinstellungen geändert',
    },
    book: {
      delete: 'Buch gelöscht',
    },
    author: {
      delete: 'Autor gelöscht',
    },
    series: {
      delete: 'Reihe gelöscht',
    },
    scanner: {
      rescan: 'Bibliotheksprüfung gestartet',
    },
    library: {
      repair_apply: 'Kapitel repariert',
      merge_books: 'Aufgeteilte Bücher zusammengeführt',
      import: 'Bücher importiert',
    },
  },
  viewMode: {
    tiles: 'Kacheln',
    small: 'Kleine Kacheln',
    list: 'Liste',
  },
  authorSort: {
    last_name: 'Nachname',
    first_name: 'Vorname',
    books: 'Anzahl der Bücher',
  },
  bookSort: {
    title: 'Reihe und Titel',
    author: 'Autor',
    published: 'Erstveröffentlichung',
    added: 'Hinzugefügt am',
  },
  colorScheme: {
    teal: 'Türkis',
    blue: 'Blau',
    violet: 'Violett',
    green: 'Grün',
  },
  themeMode: {
    light: 'Hell',
    dark: 'Dunkel',
    system: 'System',
  },
  sessionKind: {
    book: 'Buch',
    series: 'Reihe',
    list: 'Liste',
  },
  sessionFallbackTitle: 'Hörsitzung',
  loginHint: 'Optional. 3–32 Zeichen: Buchstaben ohne diakritische Zeichen, Ziffern, Punkt, Unterstrich und Bindestrich.',
}
