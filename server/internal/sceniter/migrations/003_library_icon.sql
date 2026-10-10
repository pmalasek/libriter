-- Ikona knihovny v menu (název ikony z nabídky webu, např. "clapperboard").
-- NULL = výchozí ikona podle typu (film / seriál).
ALTER TABLE sc_libraries ADD COLUMN icon TEXT;
