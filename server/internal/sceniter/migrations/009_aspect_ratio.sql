-- Ruční poměr stran titulu pro přehrávač: soubor bývá chybně označený (např.
-- 768×432 SAR 1:1, tedy 16:9, ale obraz je 4:3). NULL = podle souboru.
ALTER TABLE sc_movies ADD COLUMN aspect_ratio TEXT CHECK (aspect_ratio IN ('4:3', '16:9'));
ALTER TABLE sc_shows ADD COLUMN aspect_ratio TEXT CHECK (aspect_ratio IN ('4:3', '16:9'));
