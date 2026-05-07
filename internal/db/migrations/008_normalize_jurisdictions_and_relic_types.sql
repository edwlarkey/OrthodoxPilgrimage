-- Create lookup tables
CREATE TABLE jurisdictions (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    tradition TEXT NOT NULL DEFAULT 'Orthodox',
    pin_color TEXT NOT NULL DEFAULT '#530c38'
);

CREATE TABLE relic_types (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0
);

-- Seed Relic Types
INSERT INTO relic_types (name, sort_order) VALUES ('Major', 1), ('Fragment', 2), ('Secondary', 3);

-- Seed Jurisdictions from existing data
INSERT INTO jurisdictions (name, tradition, pin_color)
SELECT DISTINCT jurisdiction, 
       CASE WHEN jurisdiction LIKE '%Roman Catholic%' THEN 'Roman Catholic' ELSE 'Orthodox' END,
       CASE WHEN jurisdiction LIKE '%Roman Catholic%' THEN '#2e5a88' ELSE '#530c38' END
FROM churches 
WHERE jurisdiction IS NOT NULL AND jurisdiction != '';

-- Add foreign key columns to existing tables
ALTER TABLE churches ADD COLUMN jurisdiction_id INTEGER REFERENCES jurisdictions(id);
ALTER TABLE relics ADD COLUMN relic_type_id INTEGER REFERENCES relic_types(id);

-- Link existing data
UPDATE churches SET jurisdiction_id = (SELECT id FROM jurisdictions WHERE jurisdictions.name = churches.jurisdiction);
-- Default all existing relics to 'Fragment' (ID 2)
UPDATE relics SET relic_type_id = 2;
