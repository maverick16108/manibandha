-- кто был на созвоне (для архива записей): фиксируем участников по webhook participant_joined
CREATE TABLE IF NOT EXISTS conference_participants (
  id            serial PRIMARY KEY,
  conference_id integer NOT NULL REFERENCES conferences(id) ON DELETE CASCADE,
  identity      varchar(80) NOT NULL,
  name          varchar(255),
  is_guest      boolean NOT NULL DEFAULT false,
  joined_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (conference_id, identity)
);
CREATE INDEX IF NOT EXISTS ix_conf_participants_conf ON conference_participants(conference_id);
