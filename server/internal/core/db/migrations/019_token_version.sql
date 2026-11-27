-- Verze tokenů uživatele. Každý vydaný token nese claim „ver“; po změně hesla
-- nebo „Odhlásit všude“ se verze zvýší a všechny dřívější tokeny přestanou
-- platit. Tokeny vydané před touto migrací claim nemají a platí jako verze 0,
-- takže nasazení nikoho neodhlásí.
ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;
