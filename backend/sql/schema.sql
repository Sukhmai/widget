-- DROP TABLE IF EXISTS resources;

CREATE TABLE resources (
    id          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    url         TEXT            NOT NULL UNIQUE,
    source      TEXT            NOT NULL DEFAULT 'generic',
    metadata    JSONB           NOT NULL DEFAULT '{}',
    fetched_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
