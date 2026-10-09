-- -------------------------------------------------------------
--  014 – „není to duplicita“
-- -------------------------------------------------------------
--
-- Hlášení možných duplikátů se počítá ze stavu databáze, takže falešný nález
-- (dvě vydání téhož titulu s jiným vypravěčem) by v něm zůstal napořád a při
-- každém pohledu do administrace by znovu volal po řešení, které není.
--
-- Skupina se proto dá odmítnout. Klíčem je seznam ID knih ve skupině seřazený
-- a spojený čárkou – jakmile se složení skupiny změní (přibude třetí kopie),
-- klíč přestane sedět a skupina se objeví znovu. To je záměr: nová kopie je
-- nový nález, o kterém se má rozhodnout zvlášť.
--
-- Řádek na knihu (ne na celou skupinu) kvůli kaskádě: smazaná kniha odnese
-- i svou část odmítnutí, zbytek klíče pak přestane být úplný a odmítnutí
-- samo pozbude platnosti. Nic se nemusí uklízet ručně.

CREATE TABLE duplicate_dismissals (
  group_key    TEXT     NOT NULL,   -- ID knih skupiny, seřazená a spojená čárkou
  book_id      TEXT     NOT NULL REFERENCES books (id) ON DELETE CASCADE,
  dismissed_by TEXT     REFERENCES users (id) ON DELETE SET NULL,
  dismissed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (group_key, book_id)
);

CREATE INDEX duplicate_dismissals_book_idx ON duplicate_dismissals (book_id);
