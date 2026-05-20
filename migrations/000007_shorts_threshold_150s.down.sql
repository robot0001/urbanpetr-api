UPDATE youtube_video
SET type = 'video'::youtube_video_type
WHERE type = 'short'
  AND duration_seconds IS NOT NULL
  AND duration_seconds >= 80
  AND duration_seconds < 150;
