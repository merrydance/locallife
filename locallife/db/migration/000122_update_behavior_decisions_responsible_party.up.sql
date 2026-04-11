-- Phase3: allow platform_fallback in behavior_decisions responsible_party

ALTER TABLE behavior_decisions
  DROP CONSTRAINT IF EXISTS behavior_decisions_responsible_party_check;

ALTER TABLE behavior_decisions
  ADD CONSTRAINT behavior_decisions_responsible_party_check
  CHECK (responsible_party IN ('merchant', 'rider', 'user', 'unknown', 'platform_fallback'));
