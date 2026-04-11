-- Phase3 rollback: restore original responsible_party constraint

ALTER TABLE behavior_decisions
  DROP CONSTRAINT IF EXISTS behavior_decisions_responsible_party_check;

ALTER TABLE behavior_decisions
  ADD CONSTRAINT behavior_decisions_responsible_party_check
  CHECK (responsible_party IN ('merchant', 'rider', 'user', 'unknown'));
