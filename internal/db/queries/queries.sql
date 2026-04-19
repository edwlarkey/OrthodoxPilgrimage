-- name: ListSaints :many
SELECT * FROM saints
ORDER BY name;

-- name: CreateChurch :one
INSERT INTO churches (
    name,
    slug,
    type,
    address_text,
    city,
    state_province,
    country_code,
    latitude,
    longitude,
    jurisdiction,
    website,
    phone,
    description,
    image_url
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

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

-- name: CreateSaint :one
INSERT INTO saints (
    name,
    slug,
    feast_day,
    description,
    image_url,
    lives_url
) VALUES (
    ?, ?, ?, ?, ?, ?
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

-- name: DeleteAllChurches :exec
DELETE FROM churches;

-- name: DeleteAllSaints :exec
DELETE FROM saints;

-- name: DeleteAllRelics :exec
DELETE FROM relics;
