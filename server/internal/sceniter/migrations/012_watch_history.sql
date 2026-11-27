-- -------------------------------------------------------------
--  012 – historie sledování pro administraci
-- -------------------------------------------------------------
--
-- sc_watch_progress drží jen poslední pozici (přepisuje se a „označit jako
-- neviděné“ ji smaže) a sc_watch_log jen součet sekund za den bez titulu.
-- Kdo co a kdy sledoval, z nich zjistit nejde. Tahle tabulka je obdoba
-- listening_log z Libriteru: odkoukané sekundy po titulech a dnech.
--
-- Zapisuje se ve stejné transakci jako sc_watch_log (storage.SaveProgress),
-- a to jen tehdy, když zápis připsal nějaké odkoukané sekundy – pouhé
-- otevření titulu nebo ruční „označit jako zhlédnuté“ řádek nezaloží.
-- Smazání rozkoukanosti historii nemaže; smazání účtu nebo titulu ano.
--
-- Den je UTC datum z recorded_at události (stejně jako sc_watch_log).
-- first_at/last_at jsou nejstarší a nejnovější recorded_at toho dne, ve
-- formátu CURRENT_TIMESTAMP, aby šly porovnávat jako text (MIN, MAX).
-- Historie se plní až od nasazení této migrace; zpětně dopočítat nejde.

CREATE TABLE sc_watch_history (
    user_id    TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    movie_id   TEXT     REFERENCES sc_movies (id) ON DELETE CASCADE,
    episode_id TEXT     REFERENCES sc_episodes (id) ON DELETE CASCADE,
    day        TEXT     NOT NULL,                       -- 'YYYY-MM-DD' (UTC)
    seconds    INTEGER  NOT NULL DEFAULT 0 CHECK (seconds >= 0),
    first_at   DATETIME NOT NULL,
    last_at    DATETIME NOT NULL,
    CHECK ((movie_id IS NULL) <> (episode_id IS NULL))
);
CREATE UNIQUE INDEX sc_watch_history_movie_idx
    ON sc_watch_history (user_id, movie_id, day) WHERE movie_id IS NOT NULL;
CREATE UNIQUE INDEX sc_watch_history_episode_idx
    ON sc_watch_history (user_id, episode_id, day) WHERE episode_id IS NOT NULL;
CREATE INDEX sc_watch_history_user_day_idx ON sc_watch_history (user_id, day DESC);
