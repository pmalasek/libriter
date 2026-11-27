-- Vlastní barevné téma pro Sceniter, nezávislé na Libriteru. NULL = stejné
-- jako color_scheme (téma Libriteru), dokud si ho uživatel ve Sceniteru
-- nezmění. Režim zobrazení (theme_mode) zůstává společný.
ALTER TABLE users ADD COLUMN sceniter_color_scheme TEXT
    CHECK (sceniter_color_scheme IN ('midnight', 'emerald', 'burgundy'));
