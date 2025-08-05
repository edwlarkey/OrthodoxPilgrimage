-- queries.sql

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
