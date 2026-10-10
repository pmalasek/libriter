-- Název vložený v souboru (tag title kontejneru). Když název souboru
-- nenese číslo dílu, zkusí se z něj ("Pan casu S03x07 - 42"). NULL = soubor
-- se od zavedení sloupce neprošel ffprobe, '' = soubor žádný název nemá.
ALTER TABLE sc_media_files ADD COLUMN embedded_title TEXT;
