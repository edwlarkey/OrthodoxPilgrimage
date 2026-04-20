-- Add updated_at column to track when records were last updated.
ALTER TABLE churches ADD COLUMN updated_at DATE;
ALTER TABLE saints ADD COLUMN updated_at DATE;
