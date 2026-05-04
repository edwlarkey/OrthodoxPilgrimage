-- name: ListSaints :many
SELECT * FROM saints
ORDER BY name;

-- name: ListSaintsByStatus :many
SELECT * FROM saints
WHERE status = ?
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

-- name: ListChurchesByStatus :many
SELECT * FROM churches
WHERE status = ?
ORDER BY name;

-- name: ListChurchesInBounds :many
SELECT * FROM churches
WHERE latitude >= ? AND latitude <= ?
  AND longitude >= ? AND longitude <= ?
  AND status = 'published'
ORDER BY name;

-- name: ListChurchesBySaintSlug :many
SELECT c.* FROM churches c
JOIN relics r ON c.id = r.church_id
JOIN saints s ON r.saint_id = s.id
WHERE s.slug = ? AND c.status = 'published'
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
    status,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateChurch :one
UPDATE churches
SET name = ?,
    slug = ?,
    type = ?,
    address_text = ?,
    city = ?,
    state_province = ?,
    postal_code = ?,
    country_code = ?,
    latitude = ?,
    longitude = ?,
    jurisdiction = ?,
    website = ?,
    phone = ?,
    description = ?,
    status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteChurch :exec
DELETE FROM churches WHERE id = ?;

-- name: CreateSaint :one
INSERT INTO saints (
    name,
    slug,
    feast_day,
    description,
    lives_url,
    status,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateSaint :one
UPDATE saints
SET name = ?,
    slug = ?,
    feast_day = ?,
    description = ?,
    lives_url = ?,
    status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteSaint :exec
DELETE FROM saints WHERE id = ?;

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

-- name: CreateImage :exec
INSERT INTO images (
    church_id,
    saint_id,
    relic_church_id,
    relic_saint_id,
    url,
    alt_text,
    source,
    is_primary,
    sort_order
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: ListImagesForChurch :many
SELECT * FROM images
WHERE church_id = ?
ORDER BY sort_order, id;

-- name: ListImagesForSaint :many
SELECT * FROM images
WHERE saint_id = ?
ORDER BY sort_order, id;

-- name: ListImagesForRelic :many
SELECT * FROM images
WHERE relic_church_id = ? AND relic_saint_id = ?
ORDER BY sort_order, id;

-- name: DeleteImage :exec
DELETE FROM images WHERE id = ?;

-- name: DeleteAllImages :exec
DELETE FROM images;

-- name: CountChurches :one
SELECT count(*) FROM churches;

-- name: CountSaints :one
SELECT count(*) FROM saints;

-- name: CountRelics :one
SELECT count(*) FROM relics;

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
SELECT * FROM church_sources WHERE church_id = ?;

-- name: DeleteChurchSource :exec
DELETE FROM church_sources WHERE id = ?;

-- name: GetAdminByUsername :one
SELECT * FROM admins
WHERE username = ?;

-- name: GetAdmin :one
SELECT * FROM admins
WHERE id = ?;

-- name: CreateAdmin :one
INSERT INTO admins (
    username,
    password_hash,
    mfa_secret
) VALUES (
    ?, ?, ?
)
RETURNING *;

-- name: UpdateAdminMFA :exec
UPDATE admins
SET mfa_secret = ?, mfa_enabled = ?
WHERE id = ?;

-- name: UpdateAdminLastLogin :exec
UPDATE admins
SET last_login_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ListAllRelics :many
SELECT r.*, s.name as saint_name, c.name as church_name
FROM relics r
JOIN saints s ON r.saint_id = s.id
JOIN churches c ON r.church_id = c.id
ORDER BY c.name, s.name;

-- name: DeleteRelic :exec
DELETE FROM relics WHERE church_id = ? AND saint_id = ?;

-- name: ListRecentChurches :many
SELECT * FROM churches
ORDER BY updated_at DESC NULLS LAST, id DESC
LIMIT 5;

-- name: ListRecentSaints :many
SELECT * FROM saints
ORDER BY updated_at DESC NULLS LAST, id DESC
LIMIT 5;

-- name: ListRecentRelics :many
SELECT r.*, s.name as saint_name, c.name as church_name
FROM relics r
JOIN saints s ON r.saint_id = s.id
JOIN churches c ON r.church_id = c.id
ORDER BY c.updated_at DESC NULLS LAST
LIMIT 5;

-- name: ListAdmins :many
SELECT * FROM admins
ORDER BY username;

-- name: DeleteAdmin :exec
DELETE FROM admins WHERE id = ?;
