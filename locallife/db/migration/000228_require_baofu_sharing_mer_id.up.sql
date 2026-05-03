UPDATE baofu_account_bindings
SET sharing_mer_id = contract_no,
    updated_at = now()
WHERE open_state = 'active'
  AND (sharing_mer_id IS NULL OR length(trim(sharing_mer_id)) = 0)
  AND contract_no IS NOT NULL
  AND length(trim(contract_no)) > 0;

ALTER TABLE baofu_account_bindings
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_active_receiver_check;

ALTER TABLE baofu_account_bindings
    ADD CONSTRAINT baofu_account_bindings_active_receiver_check CHECK (
        open_state <> 'active' OR length(trim(COALESCE(sharing_mer_id, ''))) > 0
    );
