-- Fix: active should default to FALSE so ingested items are inactive until reviewed.
-- Also resets any existing rows that were imported with the wrong default.
ALTER TABLE youtube_history ALTER COLUMN active SET DEFAULT FALSE;
UPDATE youtube_history SET active = FALSE WHERE active = TRUE;
