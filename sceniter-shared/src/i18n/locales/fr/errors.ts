export default {
  sceniter: {
    disabled: 'La vidéothèque n’est pas configurée sur le serveur',
    metadata_disabled: 'Aucune source de métadonnées configurée ou installée',
    metadata_not_found: 'Titre introuvable auprès de la source',
    metadata_conflict: 'Ce titre est déjà dans la bibliothèque',
    metadata_failed: 'La source de métadonnées ne répond pas',
    not_found: 'Introuvable',
  },
  library: {
    not_found: 'Bibliothèque introuvable',
    conflict: 'Ce dossier est déjà une bibliothèque',
    root_unavailable: 'Le dossier VIDEO_ROOT n’est pas disponible',
  },
  movie: { not_found: 'Film introuvable' },
  show: { not_found: 'Série introuvable' },
  episode: { not_found: 'Épisode introuvable' },
  media: {
    not_found: 'Fichier vidéo introuvable',
    rename_conflict: 'Un fichier portant le nouveau nom existe déjà',
    primary_file: 'Ce fichier porte le film et ne peut pas être séparé',
    last_file: 'Le seul fichier d\'un film ne peut pas être supprimé',
  },
  renumber: { invalid: 'Renumérotation non valide : fichiers hors du dossier ou numéros d\'épisode en double' },
  trash: {
    invalid_path: 'Le chemin n\'est pas dans la corbeille d\'une bibliothèque',
    restore_conflict: 'Un fichier existe déjà à l\'emplacement d\'origine',
    not_found: 'Le fichier n\'est plus dans la corbeille',
  },
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
