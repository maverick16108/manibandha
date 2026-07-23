DROP TABLE IF EXISTS question_agreement_acks;
DELETE FROM app_settings WHERE key IN ('qagr_enabled', 'qagr_version', 'qagr_text');
ALTER TABLE app_settings ALTER COLUMN value TYPE varchar(255);
