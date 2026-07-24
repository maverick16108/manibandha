-- Ф1: фундамент мультитенантности «Свисток». Пространства (spaces) + space_id во все арендаторские
-- таблицы. Всё существующее принадлежит пространству «Манибандха» (id=1). Поведение не меняется.

CREATE TABLE IF NOT EXISTS spaces (
  id            serial PRIMARY KEY,
  slug          varchar(64)  NOT NULL UNIQUE,
  name          varchar(255) NOT NULL,
  type          varchar(32)  NOT NULL DEFAULT 'guru',      -- guru | education
  owner_user_id integer REFERENCES users(id) ON DELETE SET NULL,
  join_mode     varchar(16)  NOT NULL DEFAULT 'request',   -- request | open | link
  custom_domain varchar(255) UNIQUE,
  created_at    timestamptz  NOT NULL DEFAULT now()
);

-- предопределённое пространство «Манибандха» с фиксированным id=1 (владелец — текущий супер-админ)
INSERT INTO spaces (id, slug, name, type, join_mode, custom_domain, owner_user_id)
  SELECT 1, 'manibandha', 'Манибандха', 'guru', 'request', 'manibandha.ru',
         (SELECT ur.user_id FROM user_roles ur JOIN roles r ON r.id = ur.role_id
            WHERE r.is_superadmin ORDER BY ur.user_id LIMIT 1)
  WHERE NOT EXISTS (SELECT 1 FROM spaces WHERE id = 1);
SELECT setval(pg_get_serial_sequence('spaces', 'id'), GREATEST((SELECT max(id) FROM spaces), 1));

-- глобальный супер-админ (вне ролевой модели) — бэкфилл из роли is_superadmin
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_superadmin boolean NOT NULL DEFAULT false;
UPDATE users u SET is_superadmin = true
  WHERE EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
                  WHERE ur.user_id = u.id AND r.is_superadmin);

-- space_id во все арендаторские таблицы (DEFAULT 1 → существующие строки автоматически → «Манибандха»)
ALTER TABLE events         ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE disciples      ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE threads        ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE forum_sections ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE forum_topics   ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE conferences    ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE gallery_albums ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE roles          ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE drafts         ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE temples        ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;
ALTER TABLE app_settings   ADD COLUMN IF NOT EXISTS space_id integer NOT NULL DEFAULT 1 REFERENCES spaces(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS ix_events_space         ON events(space_id);
CREATE INDEX IF NOT EXISTS ix_disciples_space      ON disciples(space_id);
CREATE INDEX IF NOT EXISTS ix_threads_space        ON threads(space_id);
CREATE INDEX IF NOT EXISTS ix_forum_sections_space ON forum_sections(space_id);
CREATE INDEX IF NOT EXISTS ix_forum_topics_space   ON forum_topics(space_id);
CREATE INDEX IF NOT EXISTS ix_conferences_space    ON conferences(space_id);
CREATE INDEX IF NOT EXISTS ix_gallery_albums_space ON gallery_albums(space_id);
CREATE INDEX IF NOT EXISTS ix_roles_space          ON roles(space_id);
