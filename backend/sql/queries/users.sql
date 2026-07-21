-- name: UpsertUser :one
-- Registers (or re-points) a device's discovery key. Called on app launch once
-- the user has supplied their own phone number.
INSERT INTO users (device_id, phone_hash)
VALUES ($1, $2)
ON CONFLICT (device_id) DO UPDATE
    SET phone_hash = EXCLUDED.phone_hash, updated_at = NOW()
RETURNING *;

-- name: MatchContacts :many
-- Returns the device_id for each known phone_hash. The caller uploads hashes of
-- the numbers in its contact book; only those that exist come back.
SELECT device_id, phone_hash FROM users
WHERE phone_hash = ANY($1::bytea[]);
