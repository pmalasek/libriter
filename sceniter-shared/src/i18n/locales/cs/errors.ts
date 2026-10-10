export default {
  sceniter: {
    disabled: 'Videoknihovna na serveru není nastavená',
    metadata_disabled: 'Zdroj metadat není nastaven nebo nainstalován',
    metadata_not_found: 'Titul u zdroje nenalezen',
    metadata_conflict: 'Tento titul už v knihovně je',
    metadata_failed: 'Zdroj metadat neodpovídá',
    not_found: 'Nenalezeno',
  },
  library: {
    not_found: 'Knihovna nenalezena',
    conflict: 'Tato složka už knihovnou je',
    root_unavailable: 'Složka VIDEO_ROOT není dostupná',
  },
  movie: { not_found: 'Film nenalezen' },
  show: { not_found: 'Seriál nenalezen' },
  episode: { not_found: 'Díl nenalezen' },
  media: {
    not_found: 'Soubor s videem nenalezen',
    rename_conflict: 'Soubor s novým názvem už existuje',
    primary_file: 'Tento soubor drží film, nelze ho oddělit',
    last_file: 'Jediný soubor filmu nelze smazat',
  },
  renumber: { invalid: 'Neplatné přečíslování: soubory mimo složku nebo duplicitní čísla dílů' },
  trash: {
    invalid_path: 'Cesta nevede do koše knihovny',
    restore_conflict: 'Na původním místě už soubor je',
    not_found: 'Soubor už v koši není',
  },
  subtitle: {
    not_found: 'Titulky nenalezeny',
    unsupported: 'Tyto titulky nejde převést',
  },
  transcode: {
    busy: 'Server převádí nejvíc videí najednou',
    unavailable: 'Server neumí převádět video (chybí ffmpeg)',
    failed: 'Převod videa selhal',
    gone: 'Převod skončil, spusťte přehrávání znovu',
    pending: 'Část videa ještě není připravená',
    not_found: 'Převod nenalezen',
  },
  progress: { stale: 'Na serveru je novější pozice' },
  image: { not_found: 'Obrázek nenalezen' },
}
