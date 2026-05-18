ALTER TABLE baofu_account_bindings
    DROP CONSTRAINT IF EXISTS baofu_account_bindings_active_receiver_check;

ALTER TABLE baofu_account_bindings
    ADD CONSTRAINT baofu_account_bindings_active_receiver_check CHECK (
        open_state <> 'active' OR length(trim(COALESCE(sharing_mer_id, ''))) > 0
    );
