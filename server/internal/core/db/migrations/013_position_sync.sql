-- -------------------------------------------------------------
--  013 – dávková synchronizace pozic z offline zařízení
-- -------------------------------------------------------------
--
-- Mobil poslouchá i bez signálu a pozice odesílá až po připojení – klidně
-- hodinu starou dávku. Dosavadní pravidlo "poslední zápis vyhrává" by takovou
-- dávkou přepsalo novější pozici z jiného zařízení, a opakované odeslání téže
-- dávky (ztracená odpověď, restart) by v deníku poslechu započítalo minuty
-- dvakrát. Obojí řeší tahle migrace.
--
-- position_recorded_at je čas, kdy pozice vznikla **na klientovi**, ne kdy
-- dorazila na server. Zápis se přijme jen s novějším razítkem, takže na pořadí
-- příchodu nezáleží. Webový přehrávač razítko neposílá a server mu dosadí své
-- "teď" – pro něj se tím nic nemění.
--
-- sync_events je krátká paměť už započtených událostí: klient každé přiřadí
-- UUID a opakovaná dávka se pozná podle něj. Řádky se po 30 dnech promazávají
-- (viz PruneSyncEvents) – starší dávku už žádný klient neposílá.
--
-- Nepoužité tabulky devices a playback_positions z migrace 001 zůstávají i
-- nadále nedotčené; device_id je tady prostý text, ne cizí klíč.

ALTER TABLE play_session_items ADD COLUMN position_recorded_at DATETIME;
ALTER TABLE play_sessions      ADD COLUMN position_recorded_at DATETIME;

CREATE TABLE sync_events (
  -- UUID události přidělené klientem; podle něj se pozná opakovaná dávka.
  id         TEXT     NOT NULL PRIMARY KEY,
  user_id    TEXT     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  device_id  TEXT     NOT NULL,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX sync_events_applied_idx ON sync_events (applied_at);
