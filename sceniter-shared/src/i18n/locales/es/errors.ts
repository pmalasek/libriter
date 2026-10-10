export default {
  sceniter: {
    disabled: 'La videoteca no está configurada en el servidor',
    metadata_disabled: 'No hay ninguna fuente de metadatos configurada o instalada',
    metadata_not_found: 'La fuente no encontró el título',
    metadata_conflict: 'Este título ya está en la biblioteca',
    metadata_failed: 'La fuente de metadatos no responde',
    not_found: 'No encontrado',
  },
  library: {
    not_found: 'Biblioteca no encontrada',
    conflict: 'Esta carpeta ya es una biblioteca',
    root_unavailable: 'La carpeta VIDEO_ROOT no está disponible',
  },
  movie: { not_found: 'Película no encontrada' },
  show: { not_found: 'Serie no encontrada' },
  episode: { not_found: 'Episodio no encontrado' },
  media: {
    not_found: 'Archivo de vídeo no encontrado',
    rename_conflict: 'Ya existe un archivo con el nuevo nombre',
    primary_file: 'Este archivo sostiene la película y no se puede separar',
    last_file: 'El único archivo de una película no se puede eliminar',
  },
  renumber: { invalid: 'Renumeración no válida: archivos fuera de la carpeta o números de episodio repetidos' },
  subtitle: {
    not_found: 'Subtítulos no encontrados',
    unsupported: 'Estos subtítulos no se pueden convertir',
  },
  transcode: {
    busy: 'El servidor convierte el número máximo de vídeos',
    unavailable: 'El servidor no puede convertir vídeo (falta ffmpeg)',
    failed: 'La conversión del vídeo falló',
    gone: 'La conversión terminó, vuelve a iniciar la reproducción',
    pending: 'Esta parte del vídeo aún no está lista',
    not_found: 'Conversión no encontrada',
  },
  progress: { stale: 'Hay una posición más reciente en el servidor' },
  image: { not_found: 'Imagen no encontrada' },
}
