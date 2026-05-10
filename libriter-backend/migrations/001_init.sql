-- =============================================================
--  Libriter - inicializační skript databáze
--  PostgreSQL 15+
--  Encoding: UTF8 / Collation: cs_CZ.utf8
-- =============================================================
--
--  Vytvoření databáze (spustit jako superuser před tímto skriptem):
--
--    CREATE DATABASE libriter
--      ENCODING    'UTF8'
--      LC_COLLATE  'cs_CZ.utf8'
--      LC_CTYPE    'cs_CZ.utf8'
--      TEMPLATE    template0;
--
-- =============================================================

BEGIN;

-- -------------------------------------------------------------
--  Rozšíření
-- -------------------------------------------------------------

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";   -- uuid_generate_v4()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";    -- crypt() pro hesla
CREATE EXTENSION IF NOT EXISTS "unaccent";    -- FTS bez diakritiky


-- -------------------------------------------------------------
--  Schémata
-- -------------------------------------------------------------

CREATE SCHEMA IF NOT EXISTS library;
CREATE SCHEMA IF NOT EXISTS user_data;


-- -------------------------------------------------------------
--  Pomocná funkce pro automatické updated_at
-- -------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- =============================================================
--  SCHÉMA: library
-- =============================================================

-- -------------------------------------------------------------
--  library.authors
-- -------------------------------------------------------------

CREATE TABLE library.authors (
  id         UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
  NAME       TEXT          NOT NULL COLLATE "cs_CZ.utf8",
  bio        TEXT,
  image_path TEXT,
  created_at TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX authors_name_idx ON library.authors (NAME COLLATE "cs_CZ.utf8");


-- -------------------------------------------------------------
--  library.series
-- -------------------------------------------------------------

CREATE TABLE library.series (
  id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  title       TEXT        NOT NULL COLLATE "cs_CZ.utf8",
  description TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX series_title_idx ON library.series (title COLLATE "cs_CZ.utf8");


-- -------------------------------------------------------------
--  library.books
-- -------------------------------------------------------------

CREATE TABLE library.books (
  id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  author_id        UUID        NOT NULL REFERENCES library.authors (id) ON DELETE RESTRICT,
  series_id        UUID        REFERENCES library.series (id) ON DELETE SET NULL,
  series_position  SMALLINT,
  title            TEXT        NOT NULL COLLATE "cs_CZ.utf8",
  narrator         TEXT        COLLATE "cs_CZ.utf8",
  duration_seconds INTEGER     NOT NULL CHECK (duration_seconds > 0),
  file_path        TEXT        NOT NULL,
  cover_path       TEXT,
  LANGUAGE         CHAR(2)     NOT NULL DEFAULT 'cs',  -- ISO 639-1
  description      TEXT,
  internal_rating  SMALLINT    CHECK (internal_rating BETWEEN 1 AND 5),
  -- full-text search
  title_fts        TSVECTOR    GENERATED ALWAYS AS (
                     to_tsvector('simple', COALESCE(title, ''))
                   ) STORED,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT books_series_position_check
    CHECK (
      (series_id IS NULL AND series_position IS NULL) OR
      (series_id IS NOT NULL AND series_position IS NOT NULL)
    )
);

CREATE INDEX books_author_idx     ON library.books (author_id);
CREATE INDEX books_series_idx     ON library.books (series_id, series_position);
CREATE INDEX books_language_idx   ON library.books (LANGUAGE);
CREATE INDEX books_title_fts_idx  ON library.books USING GIN (title_fts);
CREATE INDEX books_title_idx      ON library.books (title COLLATE "cs_CZ.utf8");

CREATE TRIGGER books_updated_at
  BEFORE UPDATE ON library.books
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- -------------------------------------------------------------
--  library.chapters
-- -------------------------------------------------------------

CREATE TABLE library.chapters (
  id                   UUID      PRIMARY KEY DEFAULT uuid_generate_v4(),
  book_id              UUID      NOT NULL REFERENCES library.books (id) ON DELETE CASCADE,
  POSITION             SMALLINT  NOT NULL,
  title                TEXT      NOT NULL COLLATE "cs_CZ.utf8",
  start_offset_seconds INTEGER   NOT NULL CHECK (start_offset_seconds >= 0),
  duration_seconds     INTEGER   NOT NULL CHECK (duration_seconds > 0),

  CONSTRAINT chapters_book_position_unique UNIQUE (book_id, POSITION)
);

CREATE INDEX chapters_book_idx ON library.chapters (book_id, POSITION);


-- -------------------------------------------------------------
--  library.tags
-- -------------------------------------------------------------

CREATE TABLE library.tags (
  id   UUID  PRIMARY KEY DEFAULT uuid_generate_v4(),
  NAME TEXT  NOT NULL COLLATE "cs_CZ.utf8",

  CONSTRAINT tags_name_unique UNIQUE (NAME)
);

CREATE INDEX tags_name_idx ON library.tags (NAME COLLATE "cs_CZ.utf8");


-- -------------------------------------------------------------
--  library.book_tags  (M:N)
-- -------------------------------------------------------------

CREATE TABLE library.book_tags (
  book_id UUID NOT NULL REFERENCES library.books (id) ON DELETE CASCADE,
  tag_id  UUID NOT NULL REFERENCES library.tags  (id) ON DELETE CASCADE,

  PRIMARY KEY (book_id, tag_id)
);

CREATE INDEX book_tags_tag_idx ON library.book_tags (tag_id);


-- =============================================================
--  SCHÉMA: user_data
-- =============================================================

-- -------------------------------------------------------------
--  user_data.users
-- -------------------------------------------------------------

CREATE TABLE user_data.users (
  id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  display_name  TEXT        NOT NULL DEFAULT '',
  email         TEXT        NOT NULL,
  password_hash TEXT        NOT NULL,   -- bcrypt
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT users_email_unique UNIQUE (email)
);

CREATE INDEX users_email_idx ON user_data.users (lower(email));

CREATE TRIGGER users_updated_at
  BEFORE UPDATE ON user_data.users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- -------------------------------------------------------------
--  user_data.devices
-- -------------------------------------------------------------

CREATE TABLE user_data.devices (
  id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id      UUID        NOT NULL REFERENCES user_data.users (id) ON DELETE CASCADE,
  NAME         TEXT        NOT NULL,
  platform     TEXT        NOT NULL CHECK (platform IN ('web', 'android', 'ios', 'desktop')),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX devices_user_idx ON user_data.devices (user_id);


-- -------------------------------------------------------------
--  user_data.playback_positions
--
--  Jeden záznam per (user, book) - aktualizuje se při poslechu
--  každých ~10 sekund nebo při pauze/zavření.
--  device_id říká, ze kterého zařízení přišel poslední update.
-- -------------------------------------------------------------

CREATE TABLE user_data.playback_positions (
  id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id          UUID        NOT NULL REFERENCES user_data.users   (id) ON DELETE CASCADE,
  book_id          UUID        NOT NULL REFERENCES library.books      (id) ON DELETE CASCADE,
  device_id        UUID        REFERENCES user_data.devices           (id) ON DELETE SET NULL,
  position_seconds INTEGER     NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),
  playback_speed   NUMERIC(3,2) NOT NULL DEFAULT 1.0
                               CHECK (playback_speed BETWEEN 0.5 AND 3.5),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT playback_positions_user_book_unique UNIQUE (user_id, book_id)
);

CREATE INDEX playback_positions_user_idx ON user_data.playback_positions (user_id);
CREATE INDEX playback_positions_updated_idx ON user_data.playback_positions (updated_at DESC);

-- updated_at řeší aplikace (při každém upsert), trigger by byl
-- zbytečná zátěž při ~10s zápisech


-- -------------------------------------------------------------
--  user_data.bookmarks
-- -------------------------------------------------------------

CREATE TABLE user_data.bookmarks (
  id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id          UUID        NOT NULL REFERENCES user_data.users    (id) ON DELETE CASCADE,
  book_id          UUID        NOT NULL REFERENCES library.books       (id) ON DELETE CASCADE,
  chapter_id       UUID        REFERENCES library.chapters             (id) ON DELETE SET NULL,
  position_seconds INTEGER     NOT NULL CHECK (position_seconds >= 0),
  note             TEXT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX bookmarks_user_book_idx ON user_data.bookmarks (user_id, book_id);


-- -------------------------------------------------------------
--  user_data.listening_sessions
--
--  Každý přehrávací session = jeden záznam.
--  ended_at je NULL pokud session ještě běží nebo nebyla řádně
--  ukončena (pád appky). seconds_listened se počítá ze
--  skutečně přehraného obsahu, ne z délky session.
-- -------------------------------------------------------------

CREATE TABLE user_data.listening_sessions (
  id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id          UUID        NOT NULL REFERENCES user_data.users (id) ON DELETE CASCADE,
  book_id          UUID        NOT NULL REFERENCES library.books   (id) ON DELETE CASCADE,
  device_id        UUID        REFERENCES user_data.devices        (id) ON DELETE SET NULL,
  seconds_listened INTEGER     NOT NULL DEFAULT 0 CHECK (seconds_listened >= 0),
  started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at         TIMESTAMPTZ,

  CONSTRAINT sessions_end_after_start
    CHECK (ended_at IS NULL OR ended_at > started_at)
);

CREATE INDEX sessions_user_idx    ON user_data.listening_sessions (user_id);
CREATE INDEX sessions_book_idx    ON user_data.listening_sessions (book_id);
CREATE INDEX sessions_started_idx ON user_data.listening_sessions (started_at DESC);


-- =============================================================
--  SCHÉMA: user_data - role a oprávnění
-- =============================================================

-- -------------------------------------------------------------
--  user_data.roles
-- -------------------------------------------------------------

CREATE TABLE user_data.roles (
  id          SMALLINT    PRIMARY KEY,
  name        TEXT        NOT NULL,
  description TEXT,

  CONSTRAINT roles_name_unique UNIQUE (name)
);

INSERT INTO user_data.roles (id, name, description) VALUES
  (1, 'admin',  'Plný přístup ke všem funkcím'),
  (2, 'editor', 'Správa obsahu knihovny'),
  (3, 'reader', 'Přístup k poslouchání knih');


-- -------------------------------------------------------------
--  user_data.permissions
-- -------------------------------------------------------------

CREATE TABLE user_data.permissions (
  id          SMALLSERIAL PRIMARY KEY,
  name        TEXT        NOT NULL,
  description TEXT,

  CONSTRAINT permissions_name_unique UNIQUE (name)
);

INSERT INTO user_data.permissions (name, description) VALUES
  ('books:read',   'Prohlížení a poslouchání knih'),
  ('books:write',  'Přidávání a editace knih'),
  ('books:delete', 'Mazání knih'),
  ('users:read',   'Prohlížení seznamu uživatelů'),
  ('users:write',  'Správa uživatelů'),
  ('users:delete', 'Mazání uživatelů');


-- -------------------------------------------------------------
--  user_data.role_permissions
-- -------------------------------------------------------------

CREATE TABLE user_data.role_permissions (
  role_id       SMALLINT NOT NULL REFERENCES user_data.roles       (id) ON DELETE CASCADE,
  permission_id SMALLINT NOT NULL REFERENCES user_data.permissions (id) ON DELETE CASCADE,

  PRIMARY KEY (role_id, permission_id)
);

-- admin: vše; editor: books:read + books:write; reader: books:read
INSERT INTO user_data.role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM   user_data.roles r, user_data.permissions p
WHERE  r.name = 'admin'
UNION ALL
SELECT r.id, p.id
FROM   user_data.roles r, user_data.permissions p
WHERE  r.name = 'editor' AND p.name IN ('books:read', 'books:write')
UNION ALL
SELECT r.id, p.id
FROM   user_data.roles r, user_data.permissions p
WHERE  r.name = 'reader' AND p.name = 'books:read';


-- -------------------------------------------------------------
--  user_data.user_roles
-- -------------------------------------------------------------

CREATE TABLE user_data.user_roles (
  user_id UUID     NOT NULL REFERENCES user_data.users (id) ON DELETE CASCADE,
  role_id SMALLINT NOT NULL REFERENCES user_data.roles (id) ON DELETE CASCADE,

  PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_roles_user_idx ON user_data.user_roles (user_id);


-- -------------------------------------------------------------
--  user_data.ratings
-- -------------------------------------------------------------

CREATE TABLE user_data.ratings (
  id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id    UUID        NOT NULL REFERENCES user_data.users (id) ON DELETE CASCADE,
  book_id    UUID        NOT NULL REFERENCES library.books   (id) ON DELETE CASCADE,
  rating     SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
  review     TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT ratings_user_book_unique UNIQUE (user_id, book_id)
);

CREATE INDEX ratings_book_idx ON user_data.ratings (book_id);

CREATE TRIGGER ratings_updated_at
  BEFORE UPDATE ON user_data.ratings
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- =============================================================
--  Dokončení
-- =============================================================

COMMIT;