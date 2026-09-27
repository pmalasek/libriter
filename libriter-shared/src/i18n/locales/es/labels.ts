export default {
  bookStatus: {
    finished: 'Terminado',
    started: 'En curso',
    none: 'Sin escuchar',
  },
  role: {
    admin: 'Administrador',
    editor: 'Editor',
    reader: 'Lector',
  },
  auditAction: {
    user: {
      create: 'Cuenta creada',
      role_change: 'Rol cambiado',
      delete: 'Cuenta eliminada',
      password_reset: 'Contraseña restablecida',
    },
    settings: {
      metadata_update: 'Fuentes de metadatos modificadas',
      registration_update: 'Ajustes de registro modificados',
    },
    book: {
      delete: 'Libro eliminado',
    },
    author: {
      delete: 'Autor eliminado',
    },
    series: {
      delete: 'Serie eliminada',
    },
    scanner: {
      rescan: 'Comprobación de la biblioteca iniciada',
    },
    library: {
      repair_apply: 'Capítulos reparados',
      merge_books: 'Libros divididos fusionados',
      import: 'Libros importados',
    },
  },
  viewMode: {
    tiles: 'Mosaico',
    small: 'Mosaico pequeño',
    list: 'Lista',
  },
  authorSort: {
    last_name: 'Apellidos',
    first_name: 'Nombre',
    books: 'Número de libros',
  },
  bookSort: {
    title: 'Serie y título',
    author: 'Autor',
    published: 'Primera publicación',
    added: 'Fecha de incorporación',
  },
  colorScheme: {
    teal: 'Verde azulado',
    blue: 'Azul',
    violet: 'Violeta',
    green: 'Verde',
  },
  themeMode: {
    light: 'Claro',
    dark: 'Oscuro',
    system: 'Sistema',
  },
  sessionKind: {
    book: 'Libro',
    series: 'Serie',
    list: 'Lista',
  },
  sessionFallbackTitle: 'Sesión de escucha',
  loginHint: 'Opcional. De 3 a 32 caracteres: letras sin tildes ni diacríticos, dígitos, punto, guion bajo y guion.',
}
