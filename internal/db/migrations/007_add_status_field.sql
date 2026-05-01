ALTER TABLE saints ADD COLUMN status TEXT NOT NULL DEFAULT 'published';
ALTER TABLE churches ADD COLUMN status TEXT NOT NULL DEFAULT 'published';

-- Update existing records if any were drafts, but default to published for current data
UPDATE saints SET status = 'published';
UPDATE churches SET status = 'published';
