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
      plugin_update: 'Plugin-Einstellungen geändert',
    },
    plugins: {
      reload: 'Plugins neu geladen',
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
    sceniter: {
      library_create: 'Videobibliothek erstellt',
      library_update: 'Videobibliothek geändert',
      library_delete: 'Videobibliothek gelöscht',
      library_reorder: 'Reihenfolge der Videobibliotheken geändert',
      scanner_rescan: 'Prüfung der Videobibliothek gestartet',
      match: 'Titel mit Metadaten verknüpft',
      unmatch: 'Metadaten-Verknüpfung des Titels aufgehoben',
      edit: 'Titel bearbeitet',
      detach_file: 'Datei vom Titel getrennt',
      renumber: 'Episoden neu nummeriert',
      trash_file: 'Datei in den Papierkorb verschoben',
      trash_restore: 'Datei aus dem Papierkorb wiederhergestellt',
      trash_delete: 'Datei endgültig aus dem Papierkorb gelöscht',
      trash_empty: 'Papierkorb geleert',
    },
  },
  listControls: {
    view: 'Ansicht',
    sortBy: 'Sortieren nach',
    ascending: 'Aufsteigend – zu absteigend wechseln',
    descending: 'Absteigend – zu aufsteigend wechseln',
  },
  viewMode: {
    tiles: 'Kacheln',
    small: 'Kleine Kacheln',
    list: 'Liste',
  },
  shelf: {
    previous: 'Nach links scrollen',
    next: 'Nach rechts scrollen',
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
