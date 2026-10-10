-- -------------------------------------------------------------
--  001 – výpisy velkých knihoven a inkrementální synchronizace
-- -------------------------------------------------------------
--
-- Výpisy knih se stránkují na serveru, takže se musí řadit v SQL a rychle –
-- přes index, ne tříděním celé knihovny (kolace czech se volá do Go). Klíče
-- řazení, které nejdou přímo ze sloupců knihy, drží odvozené sloupce:
--   sort_name   – název série, jinak název knihy (série stojí v abecedě
--                 tam, kam ji staví její název, díly pohromadě)
--   author_sort – hlavní (první) autor v katalogovém tvaru
--                 „příjmení křestní prostřední“
-- Udržují je triggery, takže platí pro každou cestu zápisu (API, scanner,
-- slučování, opravy knihovny).
--
-- Synchronizace mobilu: každá změna knihy, autora, série a stavu poslechu
-- dostane pořadové číslo rev z jediného čítače sync_rev. Klient si pamatuje
-- poslední rev a příště stáhne jen novější změny. Smazané záznamy zůstávají
-- jako tombstone v deleted_items, dokud je server neuklidí (sync_rev.min_rev
-- pak říká, od kdy už historie úplná není a klient musí stáhnout vše).

CREATE TABLE sync_rev (
  id      INTEGER PRIMARY KEY CHECK (id = 1),
  rev     INTEGER NOT NULL,
  -- Nejstarší rev, od kterého jsou tombstony úplné.
  min_rev INTEGER NOT NULL
);
INSERT INTO sync_rev (id, rev, min_rev) VALUES (1, 0, 0);

CREATE TABLE deleted_items (
  rev        INTEGER  NOT NULL PRIMARY KEY,
  kind       TEXT     NOT NULL CHECK (kind IN ('book', 'author', 'series', 'progress')),
  id         TEXT     NOT NULL,
  -- Jen u progress: čí stav poslechu zmizel.
  user_id    TEXT,
  deleted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX deleted_items_deleted_idx ON deleted_items (deleted_at);

ALTER TABLE books ADD COLUMN sort_name   TEXT    NOT NULL DEFAULT '' COLLATE czech;
ALTER TABLE books ADD COLUMN author_sort TEXT    NOT NULL DEFAULT '' COLLATE czech;
ALTER TABLE books ADD COLUMN rev         INTEGER NOT NULL DEFAULT 0;
ALTER TABLE authors       ADD COLUMN rev INTEGER NOT NULL DEFAULT 0;
ALTER TABLE series        ADD COLUMN rev INTEGER NOT NULL DEFAULT 0;
ALTER TABLE book_progress ADD COLUMN rev INTEGER NOT NULL DEFAULT 0;

-- --- naplnění stávajících dat ---

UPDATE books SET sort_name = COALESCE((SELECT s.title FROM series s WHERE s.id = books.series_id), books.title);
UPDATE books SET author_sort = COALESCE((
  SELECT TRIM(a.last_name || ' ' || a.first_name || ' ' || a.middle_name)
  FROM book_authors ba JOIN authors a ON a.id = ba.author_id
  WHERE ba.book_id = books.id ORDER BY ba.position LIMIT 1), '');

-- Stávající záznamy dostanou rev 1: klient bez rev je stáhne všechny.
UPDATE books SET rev = 1;
UPDATE authors SET rev = 1;
UPDATE series SET rev = 1;
UPDATE book_progress SET rev = 1;
UPDATE sync_rev SET rev = 1;

-- --- indexy pro řazení výpisů ---
-- Sestupné řazení projde tentýž index pozpátku. Knihy bez autora a bez roku
-- stojí na konci vzestupného řazení (výrazy (x = '') a (x IS NULL)).

CREATE INDEX books_sort_title_idx ON books (sort_name, series_position, title, id);
CREATE INDEX books_sort_author_idx ON books ((author_sort = ''), author_sort, sort_name, series_position, title, id);
CREATE INDEX books_sort_added_idx ON books (created_at, sort_name, series_position, title, id);
CREATE INDEX books_sort_published_idx ON books ((published_year IS NULL), published_year, (author_sort = ''), author_sort, sort_name, series_position, title, id);
-- Sestupně podle roku: knihy bez roku pořád na konci, při shodě roku autor vzestupně.
CREATE INDEX books_sort_published_desc_idx ON books ((published_year IS NULL), published_year DESC, (author_sort = ''), author_sort, sort_name, series_position, title, id);
CREATE INDEX books_rev_idx ON books (rev);
CREATE INDEX authors_rev_idx ON authors (rev);
CREATE INDEX series_rev_idx ON series (rev);
CREATE INDEX book_progress_rev_idx ON book_progress (user_id, rev);

-- --- triggery: klíče řazení ---

CREATE TRIGGER books_sort_name_ai AFTER INSERT ON books BEGIN
  UPDATE books SET sort_name = COALESCE((SELECT s.title FROM series s WHERE s.id = NEW.series_id), NEW.title)
  WHERE id = NEW.id;
END;

CREATE TRIGGER books_sort_name_au AFTER UPDATE OF title, series_id ON books BEGIN
  UPDATE books SET sort_name = COALESCE((SELECT s.title FROM series s WHERE s.id = NEW.series_id), NEW.title)
  WHERE id = NEW.id;
END;

CREATE TRIGGER series_sort_name_au AFTER UPDATE OF title ON series BEGIN
  UPDATE books SET sort_name = NEW.title WHERE series_id = NEW.id;
END;

CREATE TRIGGER book_authors_sort_ai AFTER INSERT ON book_authors BEGIN
  UPDATE books SET author_sort = COALESCE((
    SELECT TRIM(a.last_name || ' ' || a.first_name || ' ' || a.middle_name)
    FROM book_authors ba JOIN authors a ON a.id = ba.author_id
    WHERE ba.book_id = NEW.book_id ORDER BY ba.position LIMIT 1), '')
  WHERE id = NEW.book_id;
END;

CREATE TRIGGER book_authors_sort_au AFTER UPDATE ON book_authors BEGIN
  UPDATE books SET author_sort = COALESCE((
    SELECT TRIM(a.last_name || ' ' || a.first_name || ' ' || a.middle_name)
    FROM book_authors ba JOIN authors a ON a.id = ba.author_id
    WHERE ba.book_id = books.id ORDER BY ba.position LIMIT 1), '')
  WHERE id IN (OLD.book_id, NEW.book_id);
END;

CREATE TRIGGER book_authors_sort_ad AFTER DELETE ON book_authors BEGIN
  UPDATE books SET author_sort = COALESCE((
    SELECT TRIM(a.last_name || ' ' || a.first_name || ' ' || a.middle_name)
    FROM book_authors ba JOIN authors a ON a.id = ba.author_id
    WHERE ba.book_id = OLD.book_id ORDER BY ba.position LIMIT 1), '')
  WHERE id = OLD.book_id;
END;

-- Přejmenování autora změní i knihy, které ho nesou (a tím jejich rev).
CREATE TRIGGER authors_sort_au AFTER UPDATE OF first_name, middle_name, last_name, name ON authors BEGIN
  UPDATE books SET author_sort = COALESCE((
    SELECT TRIM(a.last_name || ' ' || a.first_name || ' ' || a.middle_name)
    FROM book_authors ba JOIN authors a ON a.id = ba.author_id
    WHERE ba.book_id = books.id ORDER BY ba.position LIMIT 1), '')
  WHERE id IN (SELECT book_id FROM book_authors WHERE author_id = NEW.id);
END;

-- --- triggery: rev pro synchronizaci ---
-- Podmínka NEW.rev = OLD.rev pustí jen změny dat, ne zápis samotného rev.

CREATE TRIGGER books_rev_ai AFTER INSERT ON books BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE books SET rev = (SELECT rev FROM sync_rev) WHERE id = NEW.id;
END;
CREATE TRIGGER books_rev_au AFTER UPDATE ON books WHEN NEW.rev = OLD.rev BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE books SET rev = (SELECT rev FROM sync_rev) WHERE id = NEW.id;
END;
CREATE TRIGGER books_rev_ad AFTER DELETE ON books BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  INSERT INTO deleted_items (rev, kind, id) SELECT rev, 'book', OLD.id FROM sync_rev;
END;

CREATE TRIGGER authors_rev_ai AFTER INSERT ON authors BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE authors SET rev = (SELECT rev FROM sync_rev) WHERE id = NEW.id;
END;
CREATE TRIGGER authors_rev_au AFTER UPDATE ON authors WHEN NEW.rev = OLD.rev BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE authors SET rev = (SELECT rev FROM sync_rev) WHERE id = NEW.id;
END;
CREATE TRIGGER authors_rev_ad AFTER DELETE ON authors BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  INSERT INTO deleted_items (rev, kind, id) SELECT rev, 'author', OLD.id FROM sync_rev;
END;

CREATE TRIGGER series_rev_ai AFTER INSERT ON series BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE series SET rev = (SELECT rev FROM sync_rev) WHERE id = NEW.id;
END;
CREATE TRIGGER series_rev_au AFTER UPDATE ON series WHEN NEW.rev = OLD.rev BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE series SET rev = (SELECT rev FROM sync_rev) WHERE id = NEW.id;
END;
CREATE TRIGGER series_rev_ad AFTER DELETE ON series BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  INSERT INTO deleted_items (rev, kind, id) SELECT rev, 'series', OLD.id FROM sync_rev;
END;

CREATE TRIGGER book_progress_rev_ai AFTER INSERT ON book_progress BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE book_progress SET rev = (SELECT rev FROM sync_rev)
  WHERE user_id = NEW.user_id AND book_id = NEW.book_id;
END;
CREATE TRIGGER book_progress_rev_au AFTER UPDATE ON book_progress WHEN NEW.rev = OLD.rev BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  UPDATE book_progress SET rev = (SELECT rev FROM sync_rev)
  WHERE user_id = NEW.user_id AND book_id = NEW.book_id;
END;
CREATE TRIGGER book_progress_rev_ad AFTER DELETE ON book_progress BEGIN
  UPDATE sync_rev SET rev = rev + 1;
  INSERT INTO deleted_items (rev, kind, id, user_id) SELECT rev, 'progress', OLD.book_id, OLD.user_id FROM sync_rev;
END;
