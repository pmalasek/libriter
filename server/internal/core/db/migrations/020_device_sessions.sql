-- Relace zařízení: každý mobilní a televizní token nese claim „jti“ s ID
-- řádku a streamovací token vydaný zařízením claim „sid“. Odvolaná relace
-- (revoked_at není NULL) zneplatní jen tokeny toho jednoho zařízení.
-- Starší tokeny bez jti se odvolat jednotlivě nedají, jen přes token_version.
CREATE TABLE device_sessions (
  id           TEXT    PRIMARY KEY,
  user_id      TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  scope        TEXT    NOT NULL CHECK (scope IN ('mobile', 'tv')),
  device_name  TEXT    NOT NULL DEFAULT '',
  created_at   INTEGER NOT NULL,   -- unix sekundy
  last_seen_at INTEGER NOT NULL,   -- unix sekundy, zapisuje se nejvýš jednou za hodinu
  revoked_at   INTEGER             -- NULL = platná relace
);

CREATE INDEX device_sessions_user ON device_sessions (user_id);
