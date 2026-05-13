ALTER TABLE youtube_history
    ADD COLUMN comment     TEXT,
    ADD COLUMN custom_tags TEXT[] NOT NULL DEFAULT '{}';
