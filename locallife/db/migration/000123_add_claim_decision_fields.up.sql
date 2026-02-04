-- Phase3: add decision fields to claims

ALTER TABLE claims
  ADD COLUMN IF NOT EXISTS decision_version TEXT,
  ADD COLUMN IF NOT EXISTS decision_reason TEXT;
