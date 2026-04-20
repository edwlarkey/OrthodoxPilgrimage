-- Add church_sources table for tracking sources of relic information.
CREATE TABLE church_sources (
    id INTEGER PRIMARY KEY,
    church_id INTEGER NOT NULL,
    source TEXT NOT NULL,
    FOREIGN KEY (church_id) REFERENCES churches(id) ON DELETE CASCADE
);

CREATE INDEX idx_church_sources_church_id ON church_sources(church_id);