-- Ф2: участники пространства + роли в контексте пространства. Супер-админ убран из ролевой модели
-- (его заменяет глобальный флаг users.is_superadmin, выставленный в Ф1).

CREATE TABLE IF NOT EXISTS space_members (
  space_id  integer     NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  user_id   integer     NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  status    varchar(16) NOT NULL DEFAULT 'active',  -- active | pending | rejected
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (space_id, user_id)
);
CREATE INDEX IF NOT EXISTS ix_space_members_user ON space_members(user_id);

-- все текущие пользователи → активные участники «Манибандхи» (сохраняем нынешний доступ)
INSERT INTO space_members (space_id, user_id, status)
  SELECT 1, id, 'active' FROM users
  ON CONFLICT (space_id, user_id) DO NOTHING;

-- убираем супер-админ-роль из ролевой модели (доступ уже перенесён на флаг users.is_superadmin в Ф1)
DELETE FROM user_roles WHERE role_id IN (SELECT id FROM roles WHERE is_superadmin);
DELETE FROM roles WHERE is_superadmin;
