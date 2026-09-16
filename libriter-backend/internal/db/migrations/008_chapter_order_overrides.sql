-- -------------------------------------------------------------
--  008 – ruční pořadí kapitol
-- -------------------------------------------------------------
--
-- Editor může pořadí kapitol knihy přeskládat ručně (soubory bez track tagu,
-- špatně číslované soubory). Samotná chapters.position by se ale ztratila při
-- opravě kapitol: ta řádky smaže a scanner je načte znovu z tagů. Ruční
-- pořadí se proto drží zvlášť podle cesty k souboru a ingest ho čte
-- přednostně před track tagy i názvem souboru.
--
-- Zastaralé řádky (soubor přejmenovaný nebo smazaný) ničemu nevadí – nikdy
-- se na ně nic nenapojí. Se smazáním knihy odejdou kaskádou.

CREATE TABLE chapter_order_overrides (
  book_id   TEXT    NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  file_path TEXT    NOT NULL,   -- relativní cesta k audio souboru od AUDIO_ROOT
  position  INTEGER NOT NULL,
  PRIMARY KEY (book_id, file_path)
);
