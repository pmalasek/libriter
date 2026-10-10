-- Kdy se automatické párování titulu naposledy nepovedlo (žádný jistý
-- výsledek hledání). Takový titul se znovu hledá až po čase, ne při každém
-- průchodu knihovnou – šetří požadavky na zdroje metadat. NULL = nezkoušeno.
ALTER TABLE sc_movies ADD COLUMN match_attempted_at DATETIME;
ALTER TABLE sc_shows ADD COLUMN match_attempted_at DATETIME;
