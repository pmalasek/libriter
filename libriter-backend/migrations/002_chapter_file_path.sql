-- =============================================================
--  002 - Přidání file_path do library.chapters
--
--  Důvod: Scanner nyní seskupuje soubory z jednoho adresáře
--  do jedné knihy. Každý soubor = jedna kapitola s vlastní cestou.
--  books.file_path = adresář (relativně k AUDIO_ROOT)
--  chapters.file_path = konkrétní audio soubor
-- =============================================================

BEGIN;

ALTER TABLE library.chapters
  ADD COLUMN IF NOT EXISTS file_path TEXT NOT NULL DEFAULT '';

-- Odeberme DEFAULT, nové záznamy musí mít cestu explicitně
ALTER TABLE library.chapters
  ALTER COLUMN file_path DROP DEFAULT;

-- Odstraní staré knihy importované starým scannerem (file_path ukazuje
-- na audio soubor místo adresáře). Bezpečně identifikovatelné příponou.
-- VOLITELNÉ – odkomentovat pro cleanup po upgradu scanneru:
--
-- DELETE FROM library.books
--   WHERE file_path ~ '\.(mp3|m4a|m4b|ogg|flac|opus|aac|wav)$';

COMMIT;
