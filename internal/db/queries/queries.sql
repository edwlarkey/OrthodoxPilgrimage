-- name: CreateChurch :one
INSERT INTO churches (
    name,
    address_text,
    city,
    state_province,
    country_code,
    latitude,
    longitude,
    jurisdiction,
    website,
    description
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: GetChurch :one
SELECT * FROM churches
WHERE id = ?;

-- name: ListChurches :many
SELECT * FROM churches
ORDER BY name;

-- name: CountChurches :one
SELECT count(*) FROM churches;

-- name: ListChurchesInBounds :many
SELECT * FROM churches
WHERE latitude >= ? AND latitude <= ?
  AND longitude >= ? AND longitude <= ?
ORDER BY name;
