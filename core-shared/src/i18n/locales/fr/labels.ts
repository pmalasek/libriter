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
      library_update: 'Paramètres de la bibliothèque modifiés',
      plugin_update: 'Modification des réglages d’un plugin',
    },
    plugins: {
      reload: 'Plugins rechargés',
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
      import: 'Livres importés',
    },
    sceniter: {
      library_create: 'Vidéothèque créée',
      library_update: 'Vidéothèque modifiée',
      library_delete: 'Vidéothèque supprimée',
      library_reorder: 'Ordre des vidéothèques modifié',
      scanner_rescan: 'Analyse de la vidéothèque lancée',
      match: 'Titre associé aux métadonnées',
      unmatch: 'Association du titre annulée',
      edit: 'Titre modifié',
      detach_file: 'Fichier détaché du titre',
      renumber: 'Épisodes renumérotés',
      trash_file: 'Fichier placé dans la corbeille',
      trash_restore: 'Fichier restauré depuis la corbeille',
      trash_delete: 'Fichier définitivement supprimé de la corbeille',
      trash_empty: 'Corbeille vidée',
    },
  },
  listControls: {
    view: 'Affichage',
    sortBy: 'Trier par',
    ascending: 'Ordre croissant – passer à l’ordre décroissant',
    descending: 'Ordre décroissant – passer à l’ordre croissant',
  },
  viewMode: {
    tiles: 'Vignettes',
    small: 'Petites vignettes',
    list: 'Liste',
  },
  shelf: {
    previous: 'Défiler vers la gauche',
    next: 'Défiler vers la droite',
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
