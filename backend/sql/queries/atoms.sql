-- name: CreateAtom :one
INSERT INTO atoms (user_id, text, resource_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAtom :one
SELECT
  a.id,
  a.user_id,
  a.text,
  a.created_at,
  a.edited_at,
  r.id         AS resource_id,
  r.url        AS resource_url,
  r.source     AS resource_source,
  r.metadata   AS resource_metadata,
  r.fetched_at AS resource_fetched_at
FROM atoms a
LEFT JOIN resources r ON r.id = a.resource_id
WHERE a.id = $1;

-- name: GetLatestAtomFromUser :one
SELECT
  a.id,
  a.user_id,
  a.text,
  a.created_at,
  a.edited_at,
  r.id         AS resource_id,
  r.url        AS resource_url,
  r.source     AS resource_source,
  r.metadata   AS resource_metadata,
  r.fetched_at AS resource_fetched_at
FROM atoms a
LEFT JOIN resources r ON r.id = a.resource_id
WHERE a.user_id = $1
ORDER BY a.created_at DESC, a.id DESC
LIMIT 1;

-- name: GetActiveAtomsFromUser :many
SELECT
  a.id,
  a.user_id,
  a.text,
  a.created_at,
  a.edited_at,
  r.id         AS resource_id,
  r.url        AS resource_url,
  r.source     AS resource_source,
  r.metadata   AS resource_metadata,
  r.fetched_at AS resource_fetched_at
FROM atoms a
LEFT JOIN resources r ON r.id = a.resource_id
WHERE a.user_id = $1
  AND a.created_at >= NOW() - INTERVAL '24 hours'
ORDER BY a.created_at DESC, a.id DESC;

-- name: GetActiveFeedForSubscriber :many
SELECT
  a.id,
  a.user_id,
  a.text,
  a.created_at,
  a.edited_at,
  r.id         AS resource_id,
  r.url        AS resource_url,
  r.source     AS resource_source,
  r.metadata   AS resource_metadata,
  r.fetched_at AS resource_fetched_at
FROM atoms a
JOIN subscriptions s ON s.target_id = a.user_id
LEFT JOIN resources r ON r.id = a.resource_id
WHERE s.subscriber_id = $1
  AND a.created_at >= NOW() - INTERVAL '24 hours'
ORDER BY a.created_at DESC, a.id DESC;

-- name: ListAtomsByUser :many
SELECT
  a.id,
  a.user_id,
  a.text,
  a.created_at,
  a.edited_at,
  r.id         AS resource_id,
  r.url        AS resource_url,
  r.source     AS resource_source,
  r.metadata   AS resource_metadata,
  r.fetched_at AS resource_fetched_at
FROM atoms a
LEFT JOIN resources r ON r.id = a.resource_id
WHERE a.user_id = @user_id
  AND (
    sqlc.narg('cursor_time')::timestamptz IS NULL
    OR (a.created_at, a.id) < (sqlc.narg('cursor_time')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY a.created_at DESC, a.id DESC
LIMIT @page_size;

-- name: UpdateAtomText :exec
UPDATE atoms
SET text = $2, edited_at = NOW()
WHERE id = $1;

-- name: DeleteAtom :exec
DELETE FROM atoms
WHERE id = $1;
