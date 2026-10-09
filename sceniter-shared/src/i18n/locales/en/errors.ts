// Chybové kódy serveru, které vrací API Sceniteru. Slučují se do
// `errors` jádra (viz core-shared/src/i18n/locales/en/errors.ts).
export default {
  sceniter: {
    disabled: 'The video library is not set up on the server',
    not_found: 'Not found',
    tmdb_disabled: 'TMDB metadata is not set up (TMDB_API_KEY)',
    tmdb_not_found: 'The title was not found in TMDB',
    tmdb_conflict: 'This title is already in the library',
    tmdb_failed: 'TMDB is not responding',
  },
  movie: { not_found: 'Movie not found' },
  show: { not_found: 'TV show not found' },
  episode: { not_found: 'Episode not found' },
  media: { not_found: 'Video file not found' },
  subtitle: {
    not_found: 'Subtitles not found',
    unsupported: 'These subtitles cannot be converted',
  },
  transcode: {
    busy: 'The server is converting the maximum number of videos',
    unavailable: 'The server cannot convert video (ffmpeg is missing)',
    failed: 'The video conversion failed',
    gone: 'The conversion ended, start playback again',
    pending: 'The video part is not ready yet',
    not_found: 'Conversion not found',
  },
  progress: { stale: 'A newer position exists on the server' },
  image: { not_found: 'Image not found' },
}
