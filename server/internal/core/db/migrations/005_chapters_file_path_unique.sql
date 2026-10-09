-- -------------------------------------------------------------
--  005 – jeden audio soubor = jedna kapitola
-- -------------------------------------------------------------
--
-- Scanner pozná už načtený soubor podle chapters.file_path, ale dosud tuto
-- jednoznačnost nic nevynucovalo: upsert kapitoly řešil konflikt v
-- (book_id, position), takže druhý disk vydání (track 1/24 na CD1 i CD2)
-- přepsal cestu kapitoly z prvního disku. Přepsaný soubor pak v DB chyběl,
-- při dalším startu se ingestoval znovu a přepsal zase ten druhý – knihy se
-- tak načítaly po každém spuštění dokola.
--
-- Unikátní index z file_path dělá skutečnou identitu kapitoly; případné
-- duplicity z dřívějších scanů se před jeho vytvořením zahodí.

DELETE FROM chapters
WHERE rowid NOT IN (SELECT MIN(rowid) FROM chapters GROUP BY file_path);

CREATE UNIQUE INDEX chapters_file_path_unique ON chapters (file_path);

DROP INDEX IF EXISTS chapters_file_path_idx;
