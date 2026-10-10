export default {
  sceniter: {
    disabled: 'Die Videobibliothek ist auf dem Server nicht eingerichtet',
    metadata_disabled: 'Keine Metadatenquelle eingerichtet oder installiert',
    metadata_not_found: 'Titel bei der Quelle nicht gefunden',
    metadata_conflict: 'Dieser Titel ist bereits in der Bibliothek',
    metadata_failed: 'Die Metadatenquelle antwortet nicht',
    not_found: 'Nicht gefunden',
  },
  library: {
    not_found: 'Bibliothek nicht gefunden',
    conflict: 'Dieser Ordner ist bereits eine Bibliothek',
    root_unavailable: 'Der Ordner VIDEO_ROOT ist nicht verfügbar',
  },
  movie: { not_found: 'Film nicht gefunden' },
  show: { not_found: 'Serie nicht gefunden' },
  episode: { not_found: 'Folge nicht gefunden' },
  media: {
    not_found: 'Videodatei nicht gefunden',
    rename_conflict: 'Eine Datei mit dem neuen Namen existiert bereits',
    primary_file: 'Diese Datei trägt den Film und kann nicht abgetrennt werden',
    last_file: 'Die einzige Datei eines Films kann nicht gelöscht werden',
  },
  renumber: { invalid: 'Ungültige Neunummerierung: Dateien außerhalb des Ordners oder doppelte Folgennummern' },
  trash: {
    invalid_path: 'Der Pfad liegt nicht im Papierkorb einer Bibliothek',
    restore_conflict: 'Am ursprünglichen Ort existiert bereits eine Datei',
    not_found: 'Die Datei ist nicht mehr im Papierkorb',
  },
  subtitle: {
    not_found: 'Untertitel nicht gefunden',
    unsupported: 'Diese Untertitel können nicht umgewandelt werden',
  },
  transcode: {
    busy: 'Der Server wandelt die maximale Anzahl Videos um',
    unavailable: 'Der Server kann kein Video umwandeln (ffmpeg fehlt)',
    failed: 'Die Umwandlung des Videos ist fehlgeschlagen',
    gone: 'Die Umwandlung ist beendet, starten Sie die Wiedergabe erneut',
    pending: 'Dieser Teil des Videos ist noch nicht bereit',
    not_found: 'Umwandlung nicht gefunden',
  },
  progress: { stale: 'Auf dem Server gibt es eine neuere Position' },
  image: { not_found: 'Bild nicht gefunden' },
}
