-- -------------------------------------------------------------
--  012 – velikost audio souboru kapitoly
-- -------------------------------------------------------------
--
-- Mobilní aplikace stahuje celou knihu do telefonu a potřebuje předem vědět,
-- kolik místa si má připravit a jak daleko je stahování. Odhad z délky a
-- bitrate je u proměnného bitrate mimo, hlavička Content-Length se dozví až
-- po otevření spojení – proto se velikost drží v databázi vedle délky.
--
-- Staré řádky mají 0 = "neznámo". Doplní je scanner při dalším průchodu nebo
-- ruční Rescan z administrace; klient bere 0 jako neznámé a použije
-- Content-Length ze stahování.

ALTER TABLE chapters ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0
  CHECK (size_bytes >= 0);
