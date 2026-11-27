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
      password_change: 'Contraseña cambiada',
      device_revoke: 'Sesión de dispositivo cerrada',
      logout_all: 'Sesión cerrada en todas partes',
    },
    settings: {
      metadata_update: 'Fuentes de metadatos modificadas',
      registration_update: 'Ajustes de registro modificados',
      library_update: 'Ajustes de la biblioteca modificados',
      plugin_update: 'Cambio de ajustes de plugin',
    },
    plugins: {
      reload: 'Plugins recargados',
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
    sceniter: {
      library_create: 'Videoteca creada',
      library_update: 'Videoteca modificada',
      library_delete: 'Videoteca eliminada',
      library_reorder: 'Orden de las videotecas cambiado',
      scanner_rescan: 'Revisión de la videoteca iniciada',
      match: 'Título emparejado con metadatos',
      unmatch: 'Emparejamiento del título anulado',
      edit: 'Título editado',
      detach_file: 'Archivo separado del título',
      renumber: 'Episodios renumerados',
      trash_file: 'Archivo movido a la papelera',
      trash_restore: 'Archivo restaurado de la papelera',
      trash_delete: 'Archivo eliminado definitivamente de la papelera',
      trash_empty: 'Papelera vaciada',
    },
  },
  listControls: {
    view: 'Vista',
    sortBy: 'Ordenar por',
    ascending: 'Ascendente: cambiar a descendente',
    descending: 'Descendente: cambiar a ascendente',
  },
  viewMode: {
    tiles: 'Mosaico',
    small: 'Mosaico pequeño',
    list: 'Lista',
  },
  shelf: {
    previous: 'Desplazar a la izquierda',
    next: 'Desplazar a la derecha',
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
