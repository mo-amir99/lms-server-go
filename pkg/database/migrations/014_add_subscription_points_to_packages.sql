-- Add subscription_points to subscription_packages
-- Created: 2025-11-18

-- Add subscription_points column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'subscription_packages' 
        AND column_name = 'subscription_points'
    ) THEN
        ALTER TABLE subscription_packages ADD COLUMN subscription_points INT;
        COMMENT ON COLUMN subscription_packages.subscription_points IS 'Number of subscription points awarded with this package';
    END IF;
END$$;
