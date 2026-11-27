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
      library_update: 'Library settings changed',
      plugin_update: 'Plugin settings change',
    },
    plugins: {
      reload: 'Plugins reloaded',
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
    sceniter: {
      library_create: 'Video library created',
      library_update: 'Video library changed',
      library_delete: 'Video library deleted',
      library_reorder: 'Video libraries reordered',
      scanner_rescan: 'Video library scan started',
      match: 'Title matched to metadata',
      unmatch: 'Title match removed',
      edit: 'Title edited',
      detach_file: 'File detached from title',
      renumber: 'Episodes renumbered',
      trash_file: 'File moved to trash',
      trash_restore: 'File restored from trash',
      trash_delete: 'File permanently deleted from trash',
      trash_empty: 'Trash emptied',
    },
  },
  listControls: {
    view: 'View',
    sortBy: 'Sort by',
    ascending: 'Ascending – switch to descending',
    descending: 'Descending – switch to ascending',
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
