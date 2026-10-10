-- Osoby (herci, režiséři, scenáristé, tvůrci) a jejich účast na titulech.
-- Nahrazuje sloupec credits_json: ten zůstává jen jako zdroj pro jednorázový
-- přenos (storage.BackfillCredits při startu) a pak je NULL.

CREATE TABLE sc_people (
    id           TEXT NOT NULL PRIMARY KEY,
    name         TEXT NOT NULL COLLATE czech,
    -- Zdroj (plugin), u kterého má osoba ID; NULL u osob známých jen jménem.
    provider     TEXT,
    provider_id  TEXT,
    -- Fotka v SCENITER_IMAGE_ROOT (jen název souboru).
    profile_path TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX sc_people_provider_idx ON sc_people (provider, provider_id) WHERE provider_id IS NOT NULL;
CREATE INDEX sc_people_name_idx ON sc_people (name);

CREATE TABLE sc_credits (
    id        INTEGER NOT NULL PRIMARY KEY,
    movie_id  TEXT REFERENCES sc_movies (id) ON DELETE CASCADE,
    show_id   TEXT REFERENCES sc_shows (id) ON DELETE CASCADE,
    person_id TEXT NOT NULL REFERENCES sc_people (id) ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('cast', 'director', 'writer', 'creator')),
    character TEXT,
    position  INTEGER NOT NULL DEFAULT 0,
    CHECK ((movie_id IS NULL) <> (show_id IS NULL))
);
CREATE INDEX sc_credits_movie_idx ON sc_credits (movie_id);
CREATE INDEX sc_credits_show_idx ON sc_credits (show_id);
CREATE INDEX sc_credits_person_idx ON sc_credits (person_id);
