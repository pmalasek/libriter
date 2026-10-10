-- Indexy pro stránkované výpisy velkých knihoven. Výpis se řadí přes index
-- a stránka z konce seznamu nestojí víc než první – bez nich by SQLite
-- pokaždé třídil celou knihovnu (kolace czech se volá do Go).
-- Pořadí sloupců odpovídá storage.titleOrder; sestupně řazené sloupce
-- jsou v indexu DESC, aby se dal projít jedním směrem.
CREATE INDEX sc_movies_title_idx  ON sc_movies (title, id);
CREATE INDEX sc_movies_lib_title_idx ON sc_movies (library_id, title, id);
CREATE INDEX sc_movies_added_idx  ON sc_movies (created_at DESC, title, id);
CREATE INDEX sc_movies_year_idx   ON sc_movies (year DESC, title, id);
CREATE INDEX sc_movies_rating_idx ON sc_movies (rating DESC, title, id);

CREATE INDEX sc_shows_title_idx  ON sc_shows (title, id);
CREATE INDEX sc_shows_lib_title_idx ON sc_shows (library_id, title, id);
CREATE INDEX sc_shows_added_idx  ON sc_shows (created_at DESC, title, id);
CREATE INDEX sc_shows_year_idx   ON sc_shows (year DESC, title, id);
CREATE INDEX sc_shows_rating_idx ON sc_shows (rating DESC, title, id);

-- Filtr žánru a výčet žánrů s tituly (primární klíč začíná titulem).
CREATE INDEX sc_movie_genres_genre_idx ON sc_movie_genres (genre_id);
CREATE INDEX sc_show_genres_genre_idx  ON sc_show_genres (genre_id);

-- Nespárované tituly v administraci.
CREATE INDEX sc_movies_match_idx ON sc_movies (match_state, match_attempted_at);
CREATE INDEX sc_shows_match_idx  ON sc_shows (match_state, match_attempted_at);
