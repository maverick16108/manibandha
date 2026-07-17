-- Полный откат схемы (используется только при явном migrate down).
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
