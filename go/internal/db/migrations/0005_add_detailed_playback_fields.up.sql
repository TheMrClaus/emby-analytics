-- Add detailed playback method fields to play_sessions table
ALTER TABLE play_sessions ADD COLUMN video_method TEXT DEFAULT 'DirectPlay';
ALTER TABLE play_sessions ADD COLUMN audio_method TEXT DEFAULT 'DirectPlay';
ALTER TABLE play_sessions ADD COLUMN video_codec_from TEXT;
ALTER TABLE play_sessions ADD COLUMN video_codec_to TEXT;
ALTER TABLE play_sessions ADD COLUMN audio_codec_from TEXT;
ALTER TABLE play_sessions ADD COLUMN audio_codec_to TEXT;

-- Add indexes for better performance on the new fields
CREATE INDEX IF NOT EXISTS idx_play_sessions_video_method ON play_sessions(video_method);
CREATE INDEX IF NOT EXISTS idx_play_sessions_audio_method ON play_sessions(audio_method);
