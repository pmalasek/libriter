export default {
  bookStatus: {
    finished: 'Terminé',
    started: 'En cours',
    none: 'Non écouté',
  },
  role: {
    admin: 'Administrateur',
    editor: 'Éditeur',
    reader: 'Lecteur',
  },
  auditAction: {
    user: {
      create: 'Compte créé',
      role_change: 'Rôle modifié',
      delete: 'Compte supprimé',
      password_reset: 'Mot de passe réinitialisé',
    },
    settings: {
      metadata_update: 'Sources de métadonnées modifiées',
      registration_update: 'Paramètres d’inscription modifiés',
    },
    book: {
      delete: 'Livre supprimé',
    },
    author: {
      delete: 'Auteur supprimé',
    },
    series: {
      delete: 'Série supprimée',
    },
    scanner: {
      rescan: 'Vérification de la bibliothèque lancée',
    },
    library: {
      repair_apply: 'Chapitres réparés',
      merge_books: 'Livres scindés fusionnés',
    },
  },
  viewMode: {
    tiles: 'Vignettes',
    small: 'Petites vignettes',
    list: 'Liste',
  },
  authorSort: {
    last_name: 'Nom de famille',
    first_name: 'Prénom',
    books: 'Nombre de livres',
  },
  bookSort: {
    title: 'Série et titre',
    author: 'Auteur',
    published: 'Première publication',
    added: 'Date d’ajout',
  },
  colorScheme: {
    teal: 'Bleu sarcelle',
    blue: 'Bleu',
    violet: 'Violet',
    green: 'Vert',
  },
  themeMode: {
    light: 'Clair',
    dark: 'Sombre',
    system: 'Système',
  },
  sessionKind: {
    book: 'Livre',
    series: 'Série',
    list: 'Liste',
  },
  sessionFallbackTitle: 'Session d’écoute',
  loginHint: 'Facultatif. De 3 à 32 caractères : lettres sans signes diacritiques, chiffres, point, tiret bas et trait d’union.',
}
