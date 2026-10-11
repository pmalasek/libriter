-- Rozpracovaná párování televizí (device flow): TV ukáže user_code, uživatel
-- ho schválí na webu a TV si za device_code vyzvedne token. V databázi, aby
-- párování přežilo restart serveru. Platnost je pár minut; prošlé řádky maže
-- služba průběžně. device_code se ukládá jen jako SHA-256 hash.
CREATE TABLE device_codes (
  device_hash  TEXT    PRIMARY KEY,
  user_code    TEXT    NOT NULL UNIQUE,
  device_name  TEXT    NOT NULL DEFAULT '',
  user_id      TEXT    REFERENCES users (id) ON DELETE CASCADE,  -- NULL = ještě neschváleno
  expires_at   INTEGER NOT NULL,                                 -- unix sekundy
  last_poll_at INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX device_codes_expires ON device_codes (expires_at);
