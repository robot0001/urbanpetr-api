CREATE TYPE youtube_video_type AS ENUM ('video', 'short');

CREATE TABLE youtube_video (
    id               BIGSERIAL          PRIMARY KEY,
    uuid             UUID               NOT NULL DEFAULT gen_random_uuid(),
    video_id         TEXT               NOT NULL,
    type             youtube_video_type NOT NULL,
    title            TEXT               NOT NULL,
    channel          TEXT,
    thumbnail_url    TEXT,
    description      TEXT,
    duration_seconds INT,
    published_at     TIMESTAMPTZ,
    view_count       BIGINT,
    like_count       BIGINT,
    tags             TEXT[],
    enriched_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX youtube_video_uuid_idx     ON youtube_video (uuid);
CREATE UNIQUE INDEX youtube_video_video_id_idx ON youtube_video (video_id);

CREATE TABLE youtube_history (
    id               BIGSERIAL   PRIMARY KEY,
    uuid             UUID        NOT NULL DEFAULT gen_random_uuid(),
    id_youtube_video BIGINT      NOT NULL REFERENCES youtube_video(id),
    watched_at       TIMESTAMPTZ NOT NULL,
    active           BOOLEAN     NOT NULL DEFAULT FALSE,
    imported_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX youtube_history_uuid_idx
    ON youtube_history (uuid);
CREATE UNIQUE INDEX youtube_history_video_watched_idx
    ON youtube_history (id_youtube_video, watched_at);
CREATE INDEX youtube_history_watched_at_idx
    ON youtube_history (watched_at);
CREATE INDEX youtube_history_active_idx
    ON youtube_history (active) WHERE active = TRUE;
