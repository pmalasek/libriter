export default {
  sceniter: {
    disabled: 'La vidéothèque n’est pas configurée sur le serveur',
    not_found: 'Introuvable',
    tmdb_disabled: 'Les métadonnées TMDB ne sont pas configurées (TMDB_API_KEY)',
    tmdb_not_found: 'Titre introuvable dans TMDB',
    tmdb_conflict: 'Ce titre est déjà dans la bibliothèque',
    tmdb_failed: 'TMDB ne répond pas',
  },
  movie: { not_found: 'Film introuvable' },
  show: { not_found: 'Série introuvable' },
  episode: { not_found: 'Épisode introuvable' },
  media: { not_found: 'Fichier vidéo introuvable' },
  subtitle: {
    not_found: 'Sous-titres introuvables',
    unsupported: 'Ces sous-titres ne peuvent pas être convertis',
  },
  transcode: {
    busy: 'Le serveur convertit le nombre maximal de vidéos',
    unavailable: 'Le serveur ne peut pas convertir la vidéo (ffmpeg manquant)',
    failed: 'La conversion de la vidéo a échoué',
    gone: 'La conversion est terminée, relancez la lecture',
    pending: 'Cette partie de la vidéo n’est pas encore prête',
    not_found: 'Conversion introuvable',
  },
  progress: { stale: 'Une position plus récente existe sur le serveur' },
  image: { not_found: 'Image introuvable' },
}
