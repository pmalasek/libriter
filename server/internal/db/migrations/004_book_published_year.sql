-- -------------------------------------------------------------
--  004 – rok vydání knihy
-- -------------------------------------------------------------
--
-- Zdroje metadat (databazeknih.cz, Google Books) vrací rok prvního vydání;
-- v seznamu knih se podle něj řadí. Nepovinné – ne každý zdroj ho zná.

ALTER TABLE books ADD COLUMN published_year INTEGER;

CREATE INDEX books_published_year_idx ON books (published_year);
