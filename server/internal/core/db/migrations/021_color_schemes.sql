-- Nová barevná témata: Midnight Blue & Copper, Emerald & Gold, Burgundy & Gold
-- nahrazují schémata teal, blue, violet a green. Volba uživatelů se převede
-- na nejbližší téma.
--
-- CHECK v SQLite nejde změnit na místě. Místo přestavby celé tabulky users
-- (odkazuje na ni ~20 cizích klíčů) se vymění jen sloupec: nový s novým
-- CHECKem, přenos hodnot, smazání starého a přejmenování. Sloupec se tím
-- přesune na konec tabulky – dotazy na users vyjmenovávají sloupce, nevadí to.
ALTER TABLE users ADD COLUMN color_scheme_v2 TEXT NOT NULL DEFAULT 'midnight'
    CHECK (color_scheme_v2 IN ('midnight', 'emerald', 'burgundy'));
UPDATE users SET color_scheme_v2 = CASE color_scheme
    WHEN 'green' THEN 'emerald'
    WHEN 'violet' THEN 'burgundy'
    ELSE 'midnight'
END;
ALTER TABLE users DROP COLUMN color_scheme;
ALTER TABLE users RENAME COLUMN color_scheme_v2 TO color_scheme;
