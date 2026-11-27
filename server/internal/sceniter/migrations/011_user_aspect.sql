-- Ruční poměr stran titulu pro jednoho uživatele. Dřív se volba zapisovala
-- do sdíleného sloupce aspect_ratio (migrace 009), takže jeden divák měnil
-- obraz všem. Sdílené sloupce zůstávají jako výchozí hodnota pro uživatele,
-- kteří si nic nezvolili; nové volby se ukládají sem.
CREATE TABLE sc_user_aspect (
    user_id      TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    movie_id     TEXT     REFERENCES sc_movies (id) ON DELETE CASCADE,
    show_id      TEXT     REFERENCES sc_shows (id) ON DELETE CASCADE,
    aspect_ratio TEXT     NOT NULL CHECK (aspect_ratio IN ('4:3', '16:9')),
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((movie_id IS NULL) <> (show_id IS NULL))
);
CREATE UNIQUE INDEX sc_user_aspect_movie_idx ON sc_user_aspect (user_id, movie_id) WHERE movie_id IS NOT NULL;
CREATE UNIQUE INDEX sc_user_aspect_show_idx  ON sc_user_aspect (user_id, show_id)  WHERE show_id IS NOT NULL;
