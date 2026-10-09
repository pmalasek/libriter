-- Volitelné přihlašovací jméno. Přihlásit se jde loginem i e-mailem, proto je
-- login unikátní bez ohledu na velikost písmen; NULL = uživatel login nemá.
ALTER TABLE users ADD COLUMN login TEXT;
CREATE UNIQUE INDEX users_login_unique ON users (lower(login)) WHERE login IS NOT NULL;
