-- кто начал запись конференции (для архива записей)
ALTER TABLE conference_recordings
  ADD COLUMN IF NOT EXISTS created_by integer REFERENCES users(id) ON DELETE SET NULL;
