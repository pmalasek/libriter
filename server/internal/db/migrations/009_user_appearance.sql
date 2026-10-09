ALTER TABLE users ADD COLUMN color_scheme TEXT NOT NULL DEFAULT 'teal'
    CHECK (color_scheme IN ('teal', 'blue', 'violet', 'green'));
ALTER TABLE users ADD COLUMN theme_mode TEXT NOT NULL DEFAULT 'system'
    CHECK (theme_mode IN ('light', 'dark', 'system'));
