-- -------------------------------------------------------------
--  006 – album tag knihy (párovací klíč scanneru)
-- -------------------------------------------------------------
--
-- Scanner páruje audio soubory ke knize podle album tagu, ale porovnával ho
-- s books.title. Ten se ale mění: import metadat i ruční editace knihu
-- přejmenují a soubory ke knize pak přestanou pasovat. Fallback na adresář to
-- maskoval, jenže ten zase slepí všechny knihy z jedné složky do té první
-- (celá série v jednom adresáři).
--
-- album_tag drží hodnotu tagu tak, jak ji scanner viděl, nezávisle na názvu
-- knihy. NULL = kniha z dřívějších scanů; k té se tag doplní, až scanner
-- narazí na soubor v jejím adresáři.

ALTER TABLE books ADD COLUMN album_tag TEXT;

CREATE INDEX books_album_tag_idx ON books (album_tag);
