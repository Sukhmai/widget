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

-- name: UpsertSubscription :exec
-- Records that subscriber_id wants to be notified when target_id posts.
-- Created implicitly whenever MatchContacts resolves a friend.
INSERT INTO subscriptions (subscriber_id, target_id)
VALUES ($1, $2)
ON CONFLICT (subscriber_id, target_id) DO NOTHING;

-- name: SetPushToken :exec
-- Stores (or clears) a device's APNs token, captured after the app registers
-- for remote notifications.
UPDATE users SET push_token = $2, updated_at = NOW() WHERE device_id = $1;

-- name: ListSubscriberPushTokens :many
-- Returns push tokens for every subscriber of target_id that has one
-- registered, so the push worker knows who to notify when target_id posts.
SELECT u.push_token FROM subscriptions s
JOIN users u ON u.device_id = s.subscriber_id
WHERE s.target_id = $1 AND u.push_token IS NOT NULL;

-- name: DeleteSubscriberHashes :exec
-- Clears subscriber_id's remembered contact-hash set, so it can be replaced
-- wholesale with the hashes from its latest MatchContacts call.
DELETE FROM subscriber_hashes WHERE subscriber_id = $1;

-- name: InsertSubscriberHashes :exec
-- Records subscriber_id's current contact-hash set, so a future signup
-- matching one of these hashes can auto-subscribe subscriber_id without it
-- having to re-sync.
INSERT INTO subscriber_hashes (subscriber_id, phone_hash)
SELECT $1, unnest($2::bytea[])
ON CONFLICT DO NOTHING;

-- name: FindSubscribersByHash :many
-- Returns every subscriber_id that previously uploaded phone_hash as a
-- contact, so a brand-new registration can auto-subscribe them immediately.
SELECT subscriber_id FROM subscriber_hashes WHERE phone_hash = $1;
