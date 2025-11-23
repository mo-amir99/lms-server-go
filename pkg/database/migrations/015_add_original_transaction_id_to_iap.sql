-- Add original_transaction_id for Apple renewal tracking
-- Created: 2025-11-18

-- Add original_transaction_id column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'iap_purchases' 
        AND column_name = 'original_transaction_id'
    ) THEN
        ALTER TABLE iap_purchases ADD COLUMN original_transaction_id VARCHAR(255);
        CREATE INDEX idx_iap_purchases_original_transaction_id ON iap_purchases(original_transaction_id);
        COMMENT ON COLUMN iap_purchases.original_transaction_id IS 'Apple: original_transaction_id (stays same across renewals). Google: same as purchase_token for consistency';
    END IF;
END$$;
