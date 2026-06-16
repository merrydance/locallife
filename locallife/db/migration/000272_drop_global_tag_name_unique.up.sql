ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_name_unique;

DROP INDEX IF EXISTS tags_name_unique;
