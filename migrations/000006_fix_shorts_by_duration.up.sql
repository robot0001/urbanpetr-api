UPDATE youtube_video
SET type = 'short'::youtube_video_type
WHERE type = 'video'
  AND duration_seconds IS NOT NULL
  AND duration_seconds < 80;
