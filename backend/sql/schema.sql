-- DROP TABLE IF EXISTS resources;

CREATE TABLE resources (
    id          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    url         TEXT            NOT NULL UNIQUE,
    source      TEXT            NOT NULL DEFAULT 'generic',
    metadata    JSONB           NOT NULL DEFAULT '{}',
    fetched_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- DROP TABLE IF EXISTS atoms;

-- An atom is a user's post: optional text and/or a resource created from a URL.
-- user_id is the author's device UUID (no auth/profiles yet) and segments atoms
-- so a friend's widget can show only that user's atoms.
CREATE TABLE atoms (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID          NOT NULL,
    text        TEXT,
    resource_id UUID          REFERENCES resources(id),
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    edited_at   TIMESTAMPTZ,
    CONSTRAINT atom_has_content CHECK (text IS NOT NULL OR resource_id IS NOT NULL)
);

CREATE INDEX ON atoms (resource_id);
CREATE INDEX ON atoms (user_id, created_at DESC, id DESC);
