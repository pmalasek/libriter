-- =============================================================
--  Libriter - inicializační schéma databáze
--  SQLite 3.35+ (RETURNING, UPSERT, okénkové funkce)
--
--  Migrace běží v transakci řízené aplikací (internal/db/migrate.go),
--  proto zde není BEGIN/COMMIT.
--
--  Konvence:
--   - id UUID jsou TEXT (36 znaků), generuje je aplikace
--   - časy jsou DATETIME v UTC; CURRENT_TIMESTAMP = "YYYY-MM-DD HH:MM:SS"
--   - updated_at nastavuje aplikace explicitně v UPDATE dotazech
--     (AFTER trigger by se nepromítl do RETURNING)
--   - COLLATE czech je vlastní kolace registrovaná v driveru (internal/db/db.go)
-- =============================================================


-- =============================================================
--  Knihovna
-- =============================================================

-- -------------------------------------------------------------
--  authors
-- -------------------------------------------------------------

CREATE TABLE authors (
  id         TEXT     PRIMARY KEY,
  name       TEXT     NOT NULL COLLATE czech,
  bio        TEXT,
  image_path TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX authors_name_idx ON authors (name);


-- -------------------------------------------------------------
--  series
-- -------------------------------------------------------------

CREATE TABLE series (
  id          TEXT     PRIMARY KEY,
  title       TEXT     NOT NULL COLLATE czech,
  description TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX series_title_idx ON series (title);


-- -------------------------------------------------------------
--  books
-- -------------------------------------------------------------

CREATE TABLE books (
  id               TEXT     PRIMARY KEY,
  author_id        TEXT     NOT NULL REFERENCES authors (id) ON DELETE RESTRICT,
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

CREATE INDEX books_author_idx    ON books (author_id);
CREATE INDEX books_series_idx    ON books (series_id, series_position);
CREATE INDEX books_language_idx  ON books (language);
CREATE INDEX books_title_idx     ON books (title);
CREATE INDEX books_file_path_idx ON books (file_path);


-- -------------------------------------------------------------
--  chapters
-- -------------------------------------------------------------

CREATE TABLE chapters (
  id                   TEXT    PRIMARY KEY,
  book_id              TEXT    NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  position             INTEGER NOT NULL,
  title                TEXT    NOT NULL COLLATE czech,
  file_path            TEXT    NOT NULL,   -- relativní cesta k audio souboru od AUDIO_ROOT
  start_offset_seconds INTEGER NOT NULL CHECK (start_offset_seconds >= 0),
  duration_seconds     INTEGER NOT NULL CHECK (duration_seconds > 0),

  CONSTRAINT chapters_book_position_unique UNIQUE (book_id, position)
);

CREATE INDEX chapters_book_idx      ON chapters (book_id, position);
CREATE INDEX chapters_file_path_idx ON chapters (file_path);


-- -------------------------------------------------------------
--  tags
-- -------------------------------------------------------------

CREATE TABLE tags (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL COLLATE czech,

  CONSTRAINT tags_name_unique UNIQUE (name)
);


-- -------------------------------------------------------------
--  book_tags  (M:N)
-- -------------------------------------------------------------

CREATE TABLE book_tags (
  book_id TEXT NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  tag_id  TEXT NOT NULL REFERENCES tags  (id) ON DELETE CASCADE,

  PRIMARY KEY (book_id, tag_id)
);

CREATE INDEX book_tags_tag_idx ON book_tags (tag_id);


-- =============================================================
--  Uživatelská data
-- =============================================================

-- -------------------------------------------------------------
--  users
-- -------------------------------------------------------------

CREATE TABLE users (
  id            TEXT     PRIMARY KEY,
  display_name  TEXT     NOT NULL DEFAULT '',
  email         TEXT     NOT NULL,
  password_hash TEXT     NOT NULL,   -- bcrypt
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- E-mail je unikátní bez ohledu na velikost písmen (login porovnává lower()).
CREATE UNIQUE INDEX users_email_unique ON users (lower(email));


-- -------------------------------------------------------------
--  devices
-- -------------------------------------------------------------

CREATE TABLE devices (
  id           TEXT     PRIMARY KEY,
  user_id      TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  name         TEXT     NOT NULL,
  platform     TEXT     NOT NULL CHECK (platform IN ('web', 'android', 'ios', 'desktop')),
  last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX devices_user_idx ON devices (user_id);


-- -------------------------------------------------------------
--  playback_positions
--
--  Jeden záznam per (user, book) - aktualizuje se při poslechu
--  každých ~10 sekund nebo při pauze/zavření.
--  device_id říká, ze kterého zařízení přišel poslední update.
-- -------------------------------------------------------------

CREATE TABLE playback_positions (
  id               TEXT     PRIMARY KEY,
  user_id          TEXT     NOT NULL REFERENCES users   (id) ON DELETE CASCADE,
  book_id          TEXT     NOT NULL REFERENCES books   (id) ON DELETE CASCADE,
  device_id        TEXT     REFERENCES devices          (id) ON DELETE SET NULL,
  position_seconds INTEGER  NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),
  playback_speed   REAL     NOT NULL DEFAULT 1.0
                            CHECK (playback_speed BETWEEN 0.5 AND 3.5),
  updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT playback_positions_user_book_unique UNIQUE (user_id, book_id)
);

CREATE INDEX playback_positions_user_idx    ON playback_positions (user_id);
CREATE INDEX playback_positions_updated_idx ON playback_positions (updated_at DESC);


-- -------------------------------------------------------------
--  bookmarks
-- -------------------------------------------------------------

CREATE TABLE bookmarks (
  id               TEXT     PRIMARY KEY,
  user_id          TEXT     NOT NULL REFERENCES users    (id) ON DELETE CASCADE,
  book_id          TEXT     NOT NULL REFERENCES books    (id) ON DELETE CASCADE,
  chapter_id       TEXT     REFERENCES chapters          (id) ON DELETE SET NULL,
  position_seconds INTEGER  NOT NULL CHECK (position_seconds >= 0),
  note             TEXT,
  created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX bookmarks_user_book_idx ON bookmarks (user_id, book_id);


-- -------------------------------------------------------------
--  listening_sessions
--
--  Každý přehrávací session = jeden záznam.
--  ended_at je NULL pokud session ještě běží nebo nebyla řádně
--  ukončena (pád appky). seconds_listened se počítá ze
--  skutečně přehraného obsahu, ne z délky session.
-- -------------------------------------------------------------

CREATE TABLE listening_sessions (
  id               TEXT     PRIMARY KEY,
  user_id          TEXT     NOT NULL REFERENCES users   (id) ON DELETE CASCADE,
  book_id          TEXT     NOT NULL REFERENCES books   (id) ON DELETE CASCADE,
  device_id        TEXT     REFERENCES devices          (id) ON DELETE SET NULL,
  seconds_listened INTEGER  NOT NULL DEFAULT 0 CHECK (seconds_listened >= 0),
  started_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at         DATETIME,

  CONSTRAINT sessions_end_after_start
    CHECK (ended_at IS NULL OR ended_at > started_at)
);

CREATE INDEX sessions_user_idx    ON listening_sessions (user_id);
CREATE INDEX sessions_book_idx    ON listening_sessions (book_id);
CREATE INDEX sessions_started_idx ON listening_sessions (started_at DESC);


-- =============================================================
--  Role a oprávnění
-- =============================================================

-- -------------------------------------------------------------
--  roles
-- -------------------------------------------------------------

CREATE TABLE roles (
  id          INTEGER PRIMARY KEY,
  name        TEXT    NOT NULL,
  description TEXT,

  CONSTRAINT roles_name_unique UNIQUE (name)
);

INSERT INTO roles (id, name, description) VALUES
  (1, 'admin',  'Plný přístup ke všem funkcím'),
  (2, 'editor', 'Správa obsahu knihovny'),
  (3, 'reader', 'Přístup k poslouchání knih');


-- -------------------------------------------------------------
--  permissions
-- -------------------------------------------------------------

CREATE TABLE permissions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT    NOT NULL,
  description TEXT,

  CONSTRAINT permissions_name_unique UNIQUE (name)
);

INSERT INTO permissions (name, description) VALUES
  ('books:read',   'Prohlížení a poslouchání knih'),
  ('books:write',  'Přidávání a editace knih'),
  ('books:delete', 'Mazání knih'),
  ('users:read',   'Prohlížení seznamu uživatelů'),
  ('users:write',  'Správa uživatelů'),
  ('users:delete', 'Mazání uživatelů');


-- -------------------------------------------------------------
--  role_permissions
-- -------------------------------------------------------------

CREATE TABLE role_permissions (
  role_id       INTEGER NOT NULL REFERENCES roles       (id) ON DELETE CASCADE,
  permission_id INTEGER NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,

  PRIMARY KEY (role_id, permission_id)
);

-- admin: vše; editor: books:read + books:write; reader: books:read
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM   roles r, permissions p
WHERE  r.name = 'admin'
UNION ALL
SELECT r.id, p.id
FROM   roles r, permissions p
WHERE  r.name = 'editor' AND p.name IN ('books:read', 'books:write')
UNION ALL
SELECT r.id, p.id
FROM   roles r, permissions p
WHERE  r.name = 'reader' AND p.name = 'books:read';


-- -------------------------------------------------------------
--  user_roles
-- -------------------------------------------------------------

CREATE TABLE user_roles (
  user_id TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role_id INTEGER NOT NULL REFERENCES roles (id) ON DELETE CASCADE,

  PRIMARY KEY (user_id, role_id)
);


-- -------------------------------------------------------------
--  ratings
-- -------------------------------------------------------------

CREATE TABLE ratings (
  id         TEXT     PRIMARY KEY,
  user_id    TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  book_id    TEXT     NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  rating     INTEGER  NOT NULL CHECK (rating BETWEEN 1 AND 5),
  review     TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  CONSTRAINT ratings_user_book_unique UNIQUE (user_id, book_id)
);

CREATE INDEX ratings_book_idx ON ratings (book_id);
