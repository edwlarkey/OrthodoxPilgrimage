-- Create a dedicated images table to support multiple images per entity
CREATE TABLE images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    church_id INTEGER REFERENCES churches(id) ON DELETE CASCADE,
    saint_id INTEGER REFERENCES saints(id) ON DELETE CASCADE,
    relic_church_id INTEGER,
    relic_saint_id INTEGER,
    url TEXT NOT NULL,
    alt_text TEXT,
    source TEXT,
    is_primary BOOLEAN DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    FOREIGN KEY (relic_church_id, relic_saint_id) REFERENCES relics(church_id, saint_id) ON DELETE CASCADE,
    -- Ensure exactly one entity is linked
    CHECK (
        (church_id IS NOT NULL AND saint_id IS NULL AND relic_church_id IS NULL AND relic_saint_id IS NULL) OR
        (saint_id IS NOT NULL AND church_id IS NULL AND relic_church_id IS NULL AND relic_saint_id IS NULL) OR
        (relic_church_id IS NOT NULL AND relic_saint_id IS NOT NULL AND church_id IS NULL AND saint_id IS NULL)
    )
);

-- Clean up old columns from initial schema
ALTER TABLE churches DROP COLUMN image_url;
ALTER TABLE saints DROP COLUMN image_url;
