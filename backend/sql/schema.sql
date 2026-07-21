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

-- DROP TABLE IF EXISTS users;

-- A user maps a device's write identity (device_id, the same UUID stored in
-- atoms.user_id) to a discovery key (phone_hash). This is the ONLY place phone
-- data lives, keeping the content table PII-free. Writes stay bound to device_id
-- so an unverified phone claim can only mis-point discovery, never forge atoms.
-- No display name is stored: a matched friend is always in the viewer's own
-- contact book, so the friend's name is resolved locally on-device.
CREATE TABLE users (
    device_id   UUID          PRIMARY KEY,       -- client-supplied IDFV; no default
    phone_hash  BYTEA         UNIQUE,            -- HMAC-SHA256(E.164 number); nullable
    push_token  TEXT,                            -- APNs device token (hex); nullable until registered
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- DROP TABLE IF EXISTS subscriptions;

-- A subscription records that subscriber_id wants to be notified when
-- target_id posts. Created implicitly by MatchContacts (auto-subscribe on
-- contact match) -- there is no explicit follow/unfollow UI.
CREATE TABLE subscriptions (
    subscriber_id UUID        NOT NULL REFERENCES users(device_id) ON DELETE CASCADE,
    target_id     UUID        NOT NULL REFERENCES users(device_id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (subscriber_id, target_id)
);

CREATE INDEX ON subscriptions (target_id);
