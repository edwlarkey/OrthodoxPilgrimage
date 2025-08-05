-- 001_initial_schema.sql

-- The 'churches' table stores information about each church location.
CREATE TABLE churches (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    address_text TEXT NOT NULL,
    city TEXT NOT NULL,
    state_province TEXT NOT NULL,
    country_code TEXT NOT NULL,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    jurisdiction TEXT,
    website TEXT,
    description TEXT
);

-- The 'saints' table stores information about the saints whose relics are venerated.
CREATE TABLE saints (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    feast_day TEXT,
    description TEXT
);

-- The 'relics' table is a join table connecting churches and saints.
-- It represents that a specific church has relics of a specific saint.
CREATE TABLE relics (
    church_id INTEGER NOT NULL,
    saint_id INTEGER NOT NULL,
    description TEXT,
    PRIMARY KEY (church_id, saint_id),
    FOREIGN KEY (church_id) REFERENCES churches(id) ON DELETE CASCADE,
    FOREIGN KEY (saint_id) REFERENCES saints(id) ON DELETE CASCADE
);

-- Create indexes for faster lookups on foreign keys and location fields.
CREATE INDEX idx_relics_church_id ON relics(church_id);
CREATE INDEX idx_relics_saint_id ON relics(saint_id);
CREATE INDEX idx_churches_city ON churches(city);
CREATE INDEX idx_churches_country_code ON churches(country_code);
