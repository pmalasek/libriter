// Chybové kódy serveru, které vrací API Sceniteru. Slučují se do
// `errors` jádra (viz core-shared/src/i18n/locales/en/errors.ts).
export default {
  sceniter: {
    disabled: 'The video library is not set up on the server',
    metadata_disabled: 'No metadata source is set up or installed',
    metadata_not_found: 'The title was not found at the source',
    metadata_conflict: 'This title is already in the library',
    metadata_failed: 'The metadata source is not responding',
    not_found: 'Not found',
  },
  library: {
    not_found: 'Library not found',
    conflict: 'This folder already is a library',
    root_unavailable: 'The VIDEO_ROOT folder is not available',
  },
  movie: { not_found: 'Movie not found' },
  show: { not_found: 'TV show not found' },
  episode: { not_found: 'Episode not found' },
  media: {
    not_found: 'Video file not found',
    rename_conflict: 'A file with the new name already exists',
    primary_file: 'This file holds the movie and cannot be split off',
    last_file: 'The only file of a movie cannot be deleted',
  },
  renumber: { invalid: 'Invalid renumbering: files outside the folder or duplicate episode numbers' },
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
