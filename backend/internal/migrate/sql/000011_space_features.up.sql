-- Ф3: включаемые/выключаемые модули пространства. Эффективное состояние = дефолт по типу пространства,
-- переопределённый явными строками здесь. Отсутствие строки = дефолт (поэтому бэкфилл не нужен).

CREATE TABLE IF NOT EXISTS space_features (
  space_id integer     NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  feature  varchar(32) NOT NULL,
  enabled  boolean     NOT NULL DEFAULT true,
  PRIMARY KEY (space_id, feature)
);
