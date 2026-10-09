-- -------------------------------------------------------------
--  011 – deník poslechu a stav knih
-- -------------------------------------------------------------
--
-- play_sessions z migrace 010 drží jen stav přehrávače: co je rozposlouchané
-- a kde. Jakmile si uživatel poslech smaže, nezůstane po něm nic, a u série
-- není poznat, které díly už slyšel. Tahle migrace přidává dvě trvalé
-- tabulky pro administraci a pro stav knihy v knihovně.
--
-- listening_log: kolik sekund obsahu uživatel u které knihy za den
-- odposlouchal. Přírůstky posílá přehrávač spolu se zápisem pozice
-- (listened_seconds v PUT /sessions/{id}/position) – sčítá jen plynulý posun
-- mezi dvěma událostmi timeupdate, převíjení a výměnu souboru nepočítá.
-- Počítají se sekundy obsahu, ne času u sluchátek: hodina poslechu při 2×
-- rychlosti připíše dvě hodiny. Ztracený zápis stojí nejvýš ~10 s.
--
-- Den je datum v UTC (date('now')): pro server jednoduché a jednoznačné,
-- kolem půlnoci se pár sekund připíše sousednímu dni.
--
-- book_progress: stav knihy u uživatele. Řádek vzniká prvním zápisem pozice
-- (rozposlouchaná), finished_at se vyplní po doposlechnutí poslední kapitoly
-- nebo ručním označením. Jednou doposlechnutá kniha jí zůstane i při dalším
-- poslechu – na rozdíl od play_sessions.finished_at, které ruší další poslech.
-- Kniha bez řádku je neposlechnutá.
--
-- Tabulky playback_positions a listening_sessions z první migrace zůstávají
-- dál nepoužité – mají jinou granularitu a přepoužití by nic neušetřilo.

CREATE TABLE listening_log (
  user_id          TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  book_id          TEXT     NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  day              TEXT     NOT NULL,                       -- 'YYYY-MM-DD' (UTC)
  seconds_listened INTEGER  NOT NULL DEFAULT 0 CHECK (seconds_listened >= 0),
  first_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (user_id, book_id, day)
);

CREATE INDEX listening_log_user_day_idx ON listening_log (user_id, day DESC);

CREATE TABLE book_progress (
  user_id     TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  book_id     TEXT     NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  -- Doposlechnuto; NULL = zatím jen rozposlouchaná.
  finished_at DATETIME,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (user_id, book_id)
);

CREATE INDEX book_progress_user_idx ON book_progress (user_id, updated_at DESC);
