-- Původ plakátu a pozadí: '' = ze zdroje metadat, 'manual' = nahrál editor.
-- Ruční obrázek obnova metadat ani přepárování nepřepíše (viz ApplyMovieMeta).
ALTER TABLE sc_movies ADD COLUMN poster_source TEXT NOT NULL DEFAULT '';
ALTER TABLE sc_movies ADD COLUMN backdrop_source TEXT NOT NULL DEFAULT '';
ALTER TABLE sc_shows ADD COLUMN poster_source TEXT NOT NULL DEFAULT '';
ALTER TABLE sc_shows ADD COLUMN backdrop_source TEXT NOT NULL DEFAULT '';
