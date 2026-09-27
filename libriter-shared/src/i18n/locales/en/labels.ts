export default {
  bookStatus: {
    finished: 'Finished',
    started: 'In progress',
    none: 'Not listened',
  },
  role: {
    admin: 'Administrator',
    editor: 'Editor',
    reader: 'Reader',
  },
  auditAction: {
    user: {
      create: 'Account created',
      role_change: 'Role changed',
      delete: 'Account deleted',
      password_reset: 'Password reset',
    },
    settings: {
      metadata_update: 'Metadata sources changed',
      registration_update: 'Registration settings changed',
    },
    book: {
      delete: 'Book deleted',
    },
    author: {
      delete: 'Author deleted',
    },
    series: {
      delete: 'Series deleted',
    },
    scanner: {
      rescan: 'Library check started',
    },
    library: {
      repair_apply: 'Chapters repaired',
      merge_books: 'Split books merged',
      import: 'Books imported',
    },
  },
  viewMode: {
    tiles: 'Tiles',
    small: 'Small tiles',
    list: 'List',
  },
  authorSort: {
    last_name: 'Last name',
    first_name: 'First name',
    books: 'Number of books',
  },
  bookSort: {
    title: 'Series and title',
    author: 'Author',
    published: 'First published',
    added: 'Date added',
  },
  colorScheme: {
    teal: 'Teal',
    blue: 'Blue',
    violet: 'Violet',
    green: 'Green',
  },
  themeMode: {
    light: 'Light',
    dark: 'Dark',
    system: 'System',
  },
  sessionKind: {
    book: 'Book',
    series: 'Series',
    list: 'List',
  },
  sessionFallbackTitle: 'Listening',
  loginHint: 'Optional. 3–32 characters: letters without diacritics, digits, dot, underscore and hyphen.',
}
