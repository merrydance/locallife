-- Drop merchant settlement adjustments

DROP INDEX IF EXISTS merchant_settlement_adjustments_related_type_id_type_idx;
DROP INDEX IF EXISTS merchant_settlement_adjustments_merchant_created_idx;

DROP TABLE IF EXISTS merchant_settlement_adjustments;
