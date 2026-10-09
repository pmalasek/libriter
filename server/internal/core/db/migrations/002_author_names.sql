-- =============================================================
--  Libriter – rozdělené jméno autora + více autorů na knihu
--
--  1) authors dostávají first_name / middle_name / last_name;
--     sloupec name zůstává jako celé jméno (udržuje ho aplikace)
--  2) vazba kniha–autor se přesouvá z books.author_id do book_authors (M:N)
--
--  Migrace běží v transakci řízené aplikací (internal/db/migrate.go)
--  s vypnutou kontrolou cizích klíčů – books se kvůli zrušení author_id
--  přestavuje (SQLite neumí DROP COLUMN u sloupce s cizím klíčem).
-- =============================================================


-- -------------------------------------------------------------
--  1. Rozdělené jméno autora
-- -------------------------------------------------------------

ALTER TABLE authors ADD COLUMN first_name  TEXT NOT NULL DEFAULT '' COLLATE czech;
ALTER TABLE authors ADD COLUMN middle_name TEXT NOT NULL DEFAULT '' COLLATE czech;
ALTER TABLE authors ADD COLUMN last_name   TEXT NOT NULL DEFAULT '' COLLATE czech;

-- Rozparsuje stávající celá jména. Podporované tvary:
--   "Jan Amos Komenský"  → Jan / Amos / Komenský
--   "Komenský, Jan Amos" → Jan / Amos / Komenský
--   "Homér"              → "" / "" / Homér
--
-- Poslední slovo se získává trikem rtrim(n, replace(n, ' ', '')): množina
-- znaků bez mezer odřízne od konce celé poslední slovo, zbyde text s mezerou
-- na konci – jeho délka je pozicí, kde příjmení začíná.
WITH base AS (
  SELECT id,
         trim(name)             AS n,
         instr(trim(name), ',') AS comma
  FROM   authors
),
split AS (
  SELECT id,
         CASE WHEN comma > 0
              THEN trim(substr(n, 1, comma - 1))
              ELSE substr(n, length(rtrim(n, replace(n, ' ', ''))) + 1)
         END AS last_name,
         CASE WHEN comma > 0
              THEN trim(substr(n, comma + 1))
              ELSE trim(substr(n, 1, length(rtrim(n, replace(n, ' ', '')))))
         END AS given
  FROM   base
),
parts AS (
  SELECT id,
         last_name,
         CASE WHEN instr(given, ' ') > 0
              THEN substr(given, 1, instr(given, ' ') - 1)
              ELSE given
         END AS first_name,
         CASE WHEN instr(given, ' ') > 0
              THEN trim(substr(given, instr(given, ' ') + 1))
              ELSE ''
         END AS middle_name
  FROM   split
)
UPDATE authors
SET    first_name  = parts.first_name,
       middle_name = parts.middle_name,
       last_name   = parts.last_name
FROM   parts
WHERE  parts.id = authors.id;

-- Zástupné jméno pro neznámého autora se nerozděluje.
UPDATE authors
SET    first_name = '', middle_name = '', last_name = name
WHERE  name = 'Neznámý autor';

-- Sjednotí name s rozdělenými částmi (mění se u tvaru "Příjmení, Křestní").
UPDATE authors
SET    name = trim(replace(first_name || ' ' || middle_name || ' ' || last_name, '  ', ' '));


-- -------------------------------------------------------------
--  2. books bez author_id + book_authors (M:N)
--
--  SQLite neumí DROP COLUMN u sloupce s cizím klíčem, tabulka se proto
--  přestavuje. Nová tabulka se vytvoří pod dočasným názvem a přejmenuje
--  se až po zahození původní – přejmenování opačným směrem by přepsalo
--  REFERENCES books v chapters, bookmarks a dalších tabulkách.
-- -------------------------------------------------------------

CREATE TABLE books_new (
  id               TEXT     PRIMARY KEY,
  series_id        TEXT     REFERENCES series (id) ON DELETE SET NULL,
  series_position  INTEGER,
  title            TEXT     NOT NULL COLLATE czech,
  narrator         TEXT     COLLATE czech,
  duration_seconds INTEGER  NOT NULL CHECK (duration_seconds > 0),
  file_path        TEXT     NOT NULL,   -- relativní cesta k adresáři od AUDIO_ROOT
  cover_path       TEXT,
  language         TEXT     NOT NULL DEFAULT 'cs',  -- ISO 639-1
  description      TEXT,
  internal_rating  INTEGER  CHECK (internal_rating BETWEEN 1 AND 5),
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT books_series_position_check
    CHECK (
      (series_id IS NULL AND series_position IS NULL) OR
      (series_id IS NOT NULL AND series_position IS NOT NULL)
    )
);

INSERT INTO books_new
  (id, series_id, series_position, title, narrator, duration_seconds,
   file_path, cover_path, language, description, internal_rating,
   created_at, updated_at)
SELECT
   id, series_id, series_position, title, narrator, duration_seconds,
   file_path, cover_path, language, description, internal_rating,
   created_at, updated_at
FROM books;

-- position určuje pořadí autorů u knihy (1 = hlavní autor)
CREATE TABLE book_authors (
  book_id   TEXT    NOT NULL REFERENCES books   (id) ON DELETE CASCADE,
  author_id TEXT    NOT NULL REFERENCES authors (id) ON DELETE RESTRICT,
  position  INTEGER NOT NULL DEFAULT 1,

  PRIMARY KEY (book_id, author_id)
);

INSERT INTO book_authors (book_id, author_id, position)
SELECT id, author_id, 1 FROM books;

DROP TABLE books;
ALTER TABLE books_new RENAME TO books;

CREATE INDEX books_series_idx    ON books (series_id, series_position);
CREATE INDEX books_language_idx  ON books (language);
CREATE INDEX books_title_idx     ON books (title);
CREATE INDEX books_file_path_idx ON books (file_path);

CREATE INDEX book_authors_author_idx ON book_authors (author_id);
CREATE INDEX book_authors_book_idx   ON book_authors (book_id, position);


-- -------------------------------------------------------------
--  3. Sloučení autorů, kteří po rozdělení vyšli stejně
--     (např. "Čapek, Karel" a "Karel Čapek")
-- -------------------------------------------------------------

CREATE TABLE _author_canon AS
SELECT a.id AS old_id,
       (SELECT c.id
          FROM authors c
         WHERE c.first_name  = a.first_name
           AND c.middle_name = a.middle_name
           AND c.last_name   = a.last_name
         ORDER BY c.created_at, c.id
         LIMIT 1) AS new_id
FROM   authors a;

-- OR REPLACE: kdyby kniha měla oba duplicitní autory, zůstane jeden řádek.
UPDATE OR REPLACE book_authors
SET    author_id = (SELECT new_id FROM _author_canon WHERE old_id = book_authors.author_id)
WHERE  author_id IN (SELECT old_id FROM _author_canon WHERE old_id <> new_id);

DELETE FROM authors
WHERE  id IN (SELECT old_id FROM _author_canon WHERE old_id <> new_id);

DROP TABLE _author_canon;


-- -------------------------------------------------------------
--  4. Indexy nad novými sloupci
-- -------------------------------------------------------------

-- Autor je jednoznačně určen trojicí jmen – opora pro get-or-create ve scanneru.
CREATE UNIQUE INDEX authors_name_parts_unique ON authors (first_name, middle_name, last_name);

-- Řazení a hledání podle příjmení.
CREATE INDEX authors_last_name_idx ON authors (last_name, first_name);
