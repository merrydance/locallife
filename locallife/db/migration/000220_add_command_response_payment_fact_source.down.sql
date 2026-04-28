ALTER TABLE external_payment_facts
DROP CONSTRAINT external_payment_facts_source_check;

ALTER TABLE external_payment_facts
ADD CONSTRAINT external_payment_facts_source_check
CHECK (fact_source IN ('callback', 'query', 'manual_reconciliation'));