-- -------------------------------------------------------------
--  007 – nastavení za běhu a záznam administrativních akcí
-- -------------------------------------------------------------
--
-- Zdroje metadat a jejich pořadí se dosud daly změnit jen v .env a jen při
-- startu serveru (METADATA_PROVIDERS). Tabulka settings drží stejné hodnoty
-- v databázi, takže je admin mění z webového rozhraní a změna platí hned.
-- Hodnoty z prostředí zůstávají výchozí: dokud v settings nic není, čte se
-- konfigurace; jakmile admin nastavení uloží, vyhrává databáze.
--
-- audit_log zaznamenává, kdo a kdy provedl administrativní zásah (změna role,
-- smazání účtu, přenastavení zdrojů, oprava kapitol, …). E-mail se ukládá
-- jako snímek, aby záznam dával smysl i po smazání účtu, na který ukazuje.

CREATE TABLE settings (
  key        TEXT     PRIMARY KEY,
  value      TEXT     NOT NULL,   -- JSON
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE audit_log (
  id           INTEGER  PRIMARY KEY AUTOINCREMENT,
  actor_id     TEXT     REFERENCES users (id) ON DELETE SET NULL,
  actor_email  TEXT     NOT NULL DEFAULT '',   -- snímek, přežije smazání účtu
  action       TEXT     NOT NULL,
  target_type  TEXT     NOT NULL DEFAULT '',
  target_id    TEXT     NOT NULL DEFAULT '',
  target_label TEXT     NOT NULL DEFAULT '',   -- e-mail nebo název pro výpis
  details      TEXT     NOT NULL DEFAULT '{}', -- JSON
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Výpis jde vždy od nejnovějšího záznamu a stránkuje se přes id.
CREATE INDEX audit_log_id_desc_idx ON audit_log (id DESC);
