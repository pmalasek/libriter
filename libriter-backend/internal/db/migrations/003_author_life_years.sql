-- -------------------------------------------------------------
--  003 – roky narození a úmrtí autora
-- -------------------------------------------------------------
--
-- Zdroje metadat (databazeknih.cz, cbdb.cz, OpenLibrary) tyto roky dávají
-- spolehlivě a na detailu autora se hodí ("1890–1938").
--
-- Obě pole jsou nepovinná: u žijících autorů chybí rok úmrtí a u řady
-- autorů se nepodaří zjistit ani jeden.

ALTER TABLE authors ADD COLUMN birth_year INTEGER;
ALTER TABLE authors ADD COLUMN death_year INTEGER;
