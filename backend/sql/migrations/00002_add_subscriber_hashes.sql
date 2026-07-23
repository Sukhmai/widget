-- +goose Up
-- Remembers the phone hashes a subscriber's contact book contained the last
-- time it called MatchContacts, so that when someone new registers, we can
-- auto-subscribe everyone who already had them as a contact -- without
-- asking the client to re-read Contacts and re-call MatchContacts itself.
-- Replaced wholesale on every MatchContacts call (never merged), so a
-- contact removed on-device eventually drops out here too. Still just an
-- opaque hash per row, same privacy posture as `users.phone_hash`.
CREATE TABLE subscriber_hashes (
    subscriber_id UUID        NOT NULL REFERENCES users(device_id) ON DELETE CASCADE,
    phone_hash    BYTEA       NOT NULL,
    PRIMARY KEY (subscriber_id, phone_hash)
);

CREATE INDEX subscriber_hashes_phone_hash_idx ON subscriber_hashes (phone_hash);

-- +goose Down
DROP TABLE IF EXISTS subscriber_hashes;
