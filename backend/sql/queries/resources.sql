-- name: UpsertResource :one
INSERT INTO resources (url, source)
VALUES ($1, $2)
ON CONFLICT (url) DO UPDATE SET url = EXCLUDED.url
RETURNING *;

-- name: GetResourceByID :one
SELECT * FROM resources
WHERE id = $1;

-- name: UpdateResourceMetadata :one
UPDATE resources
SET metadata = $2, fetched_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListGenericHostCounts :many
-- Ranks the hosts behind generic (unrecognised) URLs so we can periodically see
-- which domains are worth a dedicated fetcher. Counts distinct stored URLs per
-- host (resources are deduped by URL), which is a fine popularity proxy.
SELECT substring(url FROM '^https?://([^/]+)') AS host, COUNT(*) AS n
FROM resources
WHERE source = 'generic'
GROUP BY host
ORDER BY n DESC;
