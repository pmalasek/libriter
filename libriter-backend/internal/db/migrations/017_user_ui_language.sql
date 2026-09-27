-- Jazyk uživatelského rozhraní. Noví uživatelé začínají v angličtině,
-- stávající zůstávají v češtině, ve které aplikaci dosud používali.
ALTER TABLE users ADD COLUMN ui_language TEXT NOT NULL DEFAULT 'en'
    CHECK (ui_language IN ('cs', 'en', 'fr', 'de', 'es'));
UPDATE users SET ui_language = 'cs';
