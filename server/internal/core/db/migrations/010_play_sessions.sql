-- -------------------------------------------------------------
--  010 – poslechové session
-- -------------------------------------------------------------
--
-- Session je to, co uživatel právě poslouchá: jedna kniha, celá série, nebo
-- ručně poskládaný seznam knih. Rozposlouchaných session může být víc naráz
-- (rozečtená detektivka večer, série na cesty) a přepíná se mezi nimi.
--
-- Pozice se drží na serveru, ne v prohlížeči: cílem je pokračovat z jiného
-- zařízení přesně tam, kde poslech skončil. Zapisuje se každých ~10 sekund
-- a při každé změně (pauza, převinutí, změna kapitoly, zavření stránky).
--
-- Tabulky playback_positions a listening_sessions z první migrace zůstávají
-- nedotčené – mají jinou sémantiku (pozice per kniha, resp. statistika
-- odposlouchaného času) a žádný kód je zatím nepoužívá.

CREATE TABLE play_sessions (
  id              TEXT NOT NULL PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  kind            TEXT NOT NULL CHECK (kind IN ('book', 'series', 'list')),
  -- book_id, resp. series_id, ze kterého session vznikla. Podle něj se pozná,
  -- že už session pro tuhle knihu/sérii existuje a má se v ní pokračovat.
  -- U kind='list' je NULL.
  source_id       TEXT,
  -- Název seznamu; u kind='book' a 'series' je NULL a rozhraní ho odvodí
  -- z knihy/série, aby přejmenování knihy neosiřelo ve staré session.
  title           TEXT,
  -- Kniha, která ze session hraje. NULL zbude po smazání knihy z knihovny –
  -- rozhraní pak začne první položkou.
  current_book_id TEXT REFERENCES books (id) ON DELETE SET NULL,
  playback_speed  REAL NOT NULL DEFAULT 1.0
                       CHECK (playback_speed BETWEEN 0.5 AND 3.0),
  -- Doposlechnuto do konce. Vynuluje ho až další skutečný poslech
  -- (zápis s odposlouchanými sekundami), ne samotná změna pozice.
  finished_at     DATETIME,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX play_sessions_user_updated_idx ON play_sessions (user_id, updated_at DESC);
CREATE INDEX play_sessions_source_idx       ON play_sessions (user_id, kind, source_id);

-- -------------------------------------------------------------
--  play_session_items
--
--  Knihy session v pořadí přehrávání. Každá si nese vlastní rozposlouchanou
--  kapitolu a pozici, takže skok na jiný díl série nic neztratí a návrat
--  pokračuje tam, kde předtím skončil.
--
--  Série a seznam jsou snímek z doby vzniku session – pozdější přírůstek
--  v knihovně se do běžící session nepřidá sám, uživatel ho přidá ručně.
-- -------------------------------------------------------------

CREATE TABLE play_session_items (
  session_id       TEXT    NOT NULL REFERENCES play_sessions (id) ON DELETE CASCADE,
  book_id          TEXT    NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  position         INTEGER NOT NULL,
  -- Rozposlouchaná kapitola. NULL (kapitola zmizela při opravě knihovny)
  -- znamená začít od první kapitoly knihy.
  chapter_id       TEXT    REFERENCES chapters (id) ON DELETE SET NULL,
  -- Pozice v kapitole, ne v celé knize – kapitoly se opravou knihovny
  -- přečíslují a offset v rámci knihy by pak ukazoval jinam.
  position_seconds INTEGER NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),

  PRIMARY KEY (session_id, book_id),
  CONSTRAINT play_session_items_position_unique UNIQUE (session_id, position)
);

CREATE INDEX play_session_items_book_idx ON play_session_items (book_id);
