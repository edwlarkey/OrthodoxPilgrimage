-- name: ListSaints :many
SELECT * FROM saints
ORDER BY name;


-- name: GetChurch :one
SELECT * FROM churches
WHERE id = ?;

-- name: GetChurchBySlug :one
SELECT * FROM churches
WHERE slug = ?;

-- name: ListChurches :many
SELECT * FROM churches
ORDER BY name;

-- name: ListChurchesInBounds :many
SELECT * FROM churches
WHERE latitude >= ? AND latitude <= ?
  AND longitude >= ? AND longitude <= ?
ORDER BY name;

-- name: ListChurchesBySaintSlug :many
SELECT c.* FROM churches c
JOIN relics r ON c.id = r.church_id
JOIN saints s ON r.saint_id = s.id
WHERE s.slug = ?
ORDER BY c.name;

-- name: CreateChurch :one
INSERT INTO churches (
    name,
    slug,
    type,
    address_text,
    city,
    state_province,
    postal_code,
    country_code,
    latitude,
    longitude,
    jurisdiction,
    website,
    phone,
    description,
    image_url,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: CreateSaint :one
INSERT INTO saints (
    name,
    slug,
    feast_day,
    description,
    image_url,
    lives_url,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetSaintBySlug :one
SELECT * FROM saints
WHERE slug = ?;

-- name: CreateRelic :exec
INSERT INTO relics (
    church_id,
    saint_id,
    description
) VALUES (
    ?, ?, ?
);

-- name: ListRelicsForChurch :many
SELECT s.*, r.description as relic_description
FROM saints s
JOIN relics r ON s.id = r.saint_id
WHERE r.church_id = ?;

-- name: CountChurches :one
SELECT count(*) FROM churches;

-- name: CountSaints :one
SELECT count(*) FROM saints;

-- name: SearchSaints :many
SELECT * FROM saints
WHERE name LIKE ?
ORDER BY name
LIMIT 10;

-- name: DeleteAllChurches :exec
DELETE FROM churches;

-- name: DeleteAllSaints :exec
DELETE FROM saints;

-- name: DeleteAllRelics :exec
DELETE FROM relics;

-- name: DeleteAllSources :exec
DELETE FROM church_sources;

-- name: CreateChurchSource :exec
INSERT INTO church_sources (church_id, source) VALUES (?, ?);

-- name: ListSourcesForChurch :many
SELECT source FROM church_sources WHERE church_id = ?;
