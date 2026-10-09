-- Sceniter: filmy, seriály a jejich soubory.
--
-- Identita titulu na disku je source_key: u filmu relativní cesta k souboru
-- bez přípony (film ležící naplocho) nebo k jeho adresáři, u seriálu adresář
-- seriálu. Scanner podle něj páruje soubory s existujícími záznamy.

CREATE TABLE sc_movies (
    id             TEXT     NOT NULL PRIMARY KEY,
    source_key     TEXT     NOT NULL UNIQUE,
    title          TEXT     NOT NULL COLLATE czech,
    original_title TEXT,
    year           INTEGER,
    overview       TEXT,
    tagline        TEXT,
    runtime_min    INTEGER,
    tmdb_id        INTEGER  UNIQUE,
    imdb_id        TEXT,
    poster_path    TEXT,
    backdrop_path  TEXT,
    tmdb_rating    REAL,
    credits_json   TEXT,
    match_state    TEXT     NOT NULL DEFAULT 'unmatched'
                   CHECK (match_state IN ('unmatched', 'auto', 'manual', 'ignored')),
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sc_shows (
    id             TEXT     NOT NULL PRIMARY KEY,
    source_key     TEXT     NOT NULL UNIQUE,
    title          TEXT     NOT NULL COLLATE czech,
    original_title TEXT,
    year           INTEGER,
    overview       TEXT,
    status         TEXT,
    tmdb_id        INTEGER  UNIQUE,
    poster_path    TEXT,
    backdrop_path  TEXT,
    tmdb_rating    REAL,
    credits_json   TEXT,
    match_state    TEXT     NOT NULL DEFAULT 'unmatched'
                   CHECK (match_state IN ('unmatched', 'auto', 'manual', 'ignored')),
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sc_seasons (
    id          TEXT    NOT NULL PRIMARY KEY,
    show_id     TEXT    NOT NULL REFERENCES sc_shows (id) ON DELETE CASCADE,
    number      INTEGER NOT NULL CHECK (number >= 0),
    title       TEXT,
    overview    TEXT,
    poster_path TEXT,
    UNIQUE (show_id, number)
);

CREATE TABLE sc_episodes (
    id          TEXT     NOT NULL PRIMARY KEY,
    season_id   TEXT     NOT NULL REFERENCES sc_seasons (id) ON DELETE CASCADE,
    number      INTEGER  NOT NULL CHECK (number >= 0),
    title       TEXT,
    overview    TEXT,
    air_date    TEXT,
    still_path  TEXT,
    runtime_min INTEGER,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (season_id, number)
);

-- Jeden fyzický soubor. Film nebo díl může mít víc verzí (720p, 1080p).
CREATE TABLE sc_media_files (
    id            TEXT     NOT NULL PRIMARY KEY,
    movie_id      TEXT     REFERENCES sc_movies (id) ON DELETE CASCADE,
    episode_id    TEXT     REFERENCES sc_episodes (id) ON DELETE CASCADE,
    file_path     TEXT     NOT NULL UNIQUE,  -- relativně k VIDEO_ROOT
    size_bytes    INTEGER  NOT NULL DEFAULT 0,
    mtime         DATETIME,
    container     TEXT,
    duration_sec  REAL,
    bitrate       INTEGER,
    video_codec   TEXT,
    video_profile TEXT,
    width         INTEGER,
    height        INTEGER,
    pix_fmt       TEXT,
    probed_at     DATETIME,
    probe_error   TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((movie_id IS NULL) <> (episode_id IS NULL))
);
CREATE INDEX sc_media_files_movie_idx   ON sc_media_files (movie_id);
CREATE INDEX sc_media_files_episode_idx ON sc_media_files (episode_id);

-- Stopy uvnitř souboru podle ffprobe (idx = index streamu v souboru).
CREATE TABLE sc_streams (
    media_file_id TEXT    NOT NULL REFERENCES sc_media_files (id) ON DELETE CASCADE,
    idx           INTEGER NOT NULL,
    kind          TEXT    NOT NULL CHECK (kind IN ('video', 'audio', 'subtitle')),
    codec         TEXT,
    language      TEXT,
    title         TEXT,
    channels      INTEGER,
    is_default    INTEGER NOT NULL DEFAULT 0,
    is_forced     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (media_file_id, idx)
);

-- Titulky v samostatném souboru vedle videa (Film.srt, Film.cs.srt).
CREATE TABLE sc_external_subtitles (
    id            TEXT    NOT NULL PRIMARY KEY,
    media_file_id TEXT    NOT NULL REFERENCES sc_media_files (id) ON DELETE CASCADE,
    file_path     TEXT    NOT NULL UNIQUE,
    language      TEXT,
    format        TEXT    NOT NULL,
    is_forced     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE sc_genres (
    id      INTEGER NOT NULL PRIMARY KEY,
    tmdb_id INTEGER UNIQUE,
    name    TEXT    NOT NULL COLLATE czech
);
CREATE TABLE sc_movie_genres (
    movie_id TEXT    NOT NULL REFERENCES sc_movies (id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES sc_genres (id) ON DELETE CASCADE,
    PRIMARY KEY (movie_id, genre_id)
);
CREATE TABLE sc_show_genres (
    show_id  TEXT    NOT NULL REFERENCES sc_shows (id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES sc_genres (id) ON DELETE CASCADE,
    PRIMARY KEY (show_id, genre_id)
);

-- ---- data uživatelů ----

-- Rozkoukanost. Stejná sémantika jako pozice v Libriteru (migrace 013):
-- zapíše se jen záznam s novějším recorded_at, takže starší událost
-- z jiného zařízení novější pozici nepřepíše.
CREATE TABLE sc_watch_progress (
    user_id       TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    movie_id      TEXT     REFERENCES sc_movies (id) ON DELETE CASCADE,
    episode_id    TEXT     REFERENCES sc_episodes (id) ON DELETE CASCADE,
    media_file_id TEXT     REFERENCES sc_media_files (id) ON DELETE SET NULL,
    position_sec  REAL     NOT NULL DEFAULT 0 CHECK (position_sec >= 0),
    duration_sec  REAL,
    completed     INTEGER  NOT NULL DEFAULT 0,
    device_id     TEXT,
    recorded_at   DATETIME NOT NULL,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((movie_id IS NULL) <> (episode_id IS NULL))
);
CREATE UNIQUE INDEX sc_watch_progress_movie_idx
    ON sc_watch_progress (user_id, movie_id) WHERE movie_id IS NOT NULL;
CREATE UNIQUE INDEX sc_watch_progress_episode_idx
    ON sc_watch_progress (user_id, episode_id) WHERE episode_id IS NOT NULL;
CREATE INDEX sc_watch_progress_recent_idx ON sc_watch_progress (user_id, updated_at);

-- Odkoukané sekundy po dnech (UTC den z recorded_at), jako listening_log.
CREATE TABLE sc_watch_log (
    user_id TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day     TEXT    NOT NULL,
    seconds INTEGER NOT NULL DEFAULT 0 CHECK (seconds >= 0),
    PRIMARY KEY (user_id, day)
);

CREATE TABLE sc_watchlist (
    user_id  TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    movie_id TEXT     REFERENCES sc_movies (id) ON DELETE CASCADE,
    show_id  TEXT     REFERENCES sc_shows (id) ON DELETE CASCADE,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((movie_id IS NULL) <> (show_id IS NULL))
);
CREATE UNIQUE INDEX sc_watchlist_movie_idx ON sc_watchlist (user_id, movie_id) WHERE movie_id IS NOT NULL;
CREATE UNIQUE INDEX sc_watchlist_show_idx  ON sc_watchlist (user_id, show_id)  WHERE show_id IS NOT NULL;

CREATE TABLE sc_ratings (
    user_id    TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    movie_id   TEXT     REFERENCES sc_movies (id) ON DELETE CASCADE,
    show_id    TEXT     REFERENCES sc_shows (id) ON DELETE CASCADE,
    rating     INTEGER  NOT NULL CHECK (rating BETWEEN 1 AND 10),
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((movie_id IS NULL) <> (show_id IS NULL))
);
CREATE UNIQUE INDEX sc_ratings_movie_idx ON sc_ratings (user_id, movie_id) WHERE movie_id IS NOT NULL;
CREATE UNIQUE INDEX sc_ratings_show_idx  ON sc_ratings (user_id, show_id)  WHERE show_id IS NOT NULL;
