-- Phase3 rollback: drop decision fields from claims

ALTER TABLE claims
  DROP COLUMN IF EXISTS decision_reason,
  DROP COLUMN IF EXISTS decision_version;
