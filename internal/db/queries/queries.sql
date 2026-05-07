-- name: ListJurisdictions :many
SELECT * FROM jurisdictions ORDER BY name;

-- name: GetJurisdiction :one
SELECT * FROM jurisdictions WHERE id = ?;

-- name: CreateJurisdiction :one
INSERT INTO jurisdictions (name, tradition, pin_color) VALUES (?, ?, ?) RETURNING *;

-- name: UpdateJurisdiction :one
UPDATE jurisdictions
SET name = ?, tradition = ?, pin_color = ?
WHERE id = ?
RETURNING *;

-- name: DeleteJurisdiction :exec
DELETE FROM jurisdictions WHERE id = ?;

-- name: ListRelicTypes :many
SELECT * FROM relic_types ORDER BY sort_order;

-- name: GetRelicType :one
SELECT * FROM relic_types WHERE id = ?;

-- name: CreateRelicType :one
INSERT INTO relic_types (name, sort_order) VALUES (?, ?) RETURNING *;

-- name: UpdateRelicType :one
UPDATE relic_types
SET name = ?, sort_order = ?
WHERE id = ?
RETURNING *;

-- name: DeleteRelicType :exec
DELETE FROM relic_types WHERE id = ?;

-- name: ListSaints :many
SELECT * FROM saints
ORDER BY name;

-- name: ListSaintsByStatus :many
SELECT * FROM saints
WHERE status = ?
ORDER BY name;

-- name: GetChurch :one
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
WHERE c.id = ?;

-- name: GetChurchBySlug :one
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
WHERE c.slug = ?;

-- name: ListChurches :many
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
ORDER BY c.name;

-- name: ListChurchesByStatus :many
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
WHERE c.status = ?
ORDER BY c.name;

-- name: ListChurchesInBounds :many
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
WHERE c.latitude >= ? AND c.latitude <= ?
  AND c.longitude >= ? AND c.longitude <= ?
  AND c.status = 'published'
ORDER BY c.name;

-- name: ListChurchesBySaintSlug :many
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
JOIN relics r ON c.id = r.church_id
JOIN saints s ON r.saint_id = s.id
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
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
    jurisdiction_id,
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
    jurisdiction_id = ?,
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

-- name: GetSaint :one
SELECT * FROM saints
WHERE id = ?;

-- name: CreateRelic :exec
INSERT INTO relics (
    church_id,
    saint_id,
    relic_type_id,
    description
) VALUES (
    ?, ?, ?, ?
);

-- name: UpdateRelic :exec
UPDATE relics
SET relic_type_id = ?,
    description = ?
WHERE church_id = ? AND saint_id = ?;

-- name: ListRelicsForChurch :many
SELECT s.*, r.description as relic_description, rt.name as relic_type, rt.id as relic_type_id
FROM saints s
JOIN relics r ON s.id = r.saint_id
LEFT JOIN relic_types rt ON r.relic_type_id = rt.id
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

-- name: GetImage :one
SELECT * FROM images
WHERE id = ?;

-- name: UpdateImage :exec
UPDATE images
SET alt_text = ?,
    is_primary = ?,
    sort_order = ?
WHERE id = ?;

-- name: UnsetPrimaryImageForChurch :exec
UPDATE images SET is_primary = 0 WHERE church_id = ?;

-- name: UnsetPrimaryImageForSaint :exec
UPDATE images SET is_primary = 0 WHERE saint_id = ?;

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
SELECT r.*, s.name as saint_name, c.name as church_name, rt.name as relic_type
FROM relics r
JOIN saints s ON r.saint_id = s.id
JOIN churches c ON r.church_id = c.id
LEFT JOIN relic_types rt ON r.relic_type_id = rt.id
ORDER BY c.name, s.name;

-- name: DeleteRelic :exec
DELETE FROM relics WHERE church_id = ? AND saint_id = ?;

-- name: ListRecentChurches :many
SELECT c.*, j.name as jurisdiction_name, j.tradition, j.pin_color
FROM churches c
LEFT JOIN jurisdictions j ON c.jurisdiction_id = j.id
ORDER BY c.updated_at DESC NULLS LAST, c.id DESC
LIMIT 5;

-- name: ListRecentSaints :many
SELECT * FROM saints
ORDER BY updated_at DESC NULLS LAST, id DESC
LIMIT 5;

-- name: ListRecentRelics :many
SELECT r.*, s.name as saint_name, c.name as church_name, rt.name as relic_type
FROM relics r
JOIN saints s ON r.saint_id = s.id
JOIN churches c ON r.church_id = c.id
LEFT JOIN relic_types rt ON r.relic_type_id = rt.id
ORDER BY c.updated_at DESC NULLS LAST
LIMIT 5;

-- name: ListAdmins :many
SELECT * FROM admins
ORDER BY username;

-- name: DeleteAdmin :exec
DELETE FROM admins WHERE id = ?;
