export default {
  sceniter: {
    disabled: 'La videoteca no está configurada en el servidor',
    not_found: 'No encontrado',
    tmdb_disabled: 'Los metadatos de TMDB no están configurados (TMDB_API_KEY)',
    tmdb_not_found: 'Título no encontrado en TMDB',
    tmdb_conflict: 'Este título ya está en la biblioteca',
    tmdb_failed: 'TMDB no responde',
  },
  movie: { not_found: 'Película no encontrada' },
  show: { not_found: 'Serie no encontrada' },
  episode: { not_found: 'Episodio no encontrado' },
  media: { not_found: 'Archivo de vídeo no encontrado' },
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
