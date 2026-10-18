-- Kdy se naposledy zkoušelo doplnit metadata dílu ze zdroje. NULL = ještě
-- ne. Nový díl spárovaného seriálu (přidaný po spárování) má NULL a párování
-- mu metadata doplní při dalším průchodu knihovnou.
ALTER TABLE sc_episodes ADD COLUMN meta_checked_at DATETIME;

-- Díly s popisem už metadata mají. Díly bez popisu zůstanou NULL a jednou
-- se zkusí doplnit – tím se opraví i díly přidané před touto migrací.
UPDATE sc_episodes SET meta_checked_at = CURRENT_TIMESTAMP WHERE overview IS NOT NULL;
