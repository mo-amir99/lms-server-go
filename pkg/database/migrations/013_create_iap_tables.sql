-- IAP (In-App Purchase) Tables Migration
-- Created: 2025-11-18

-- Create iap_purchases table
CREATE TABLE IF NOT EXISTS iap_purchases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    package_id UUID NOT NULL REFERENCES subscription_packages(id) ON DELETE RESTRICT,
    store VARCHAR(20) NOT NULL CHECK (store IN ('google_play', 'app_store')),
    product_id VARCHAR(255) NOT NULL,
    purchase_token TEXT NOT NULL,
    transaction_id VARCHAR(255),
    order_id VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'validated', 'expired', 'canceled', 'refunded')),
    purchase_date TIMESTAMP NOT NULL,
    expiry_date TIMESTAMP,
    auto_renewing BOOLEAN DEFAULT false,
    original_receipt TEXT,
    validation_data JSONB,
    webhook_processed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_purchase_token UNIQUE (purchase_token, store)
);

-- Create indexes for iap_purchases
CREATE INDEX idx_iap_purchases_user_id ON iap_purchases(user_id);
CREATE INDEX idx_iap_purchases_subscription_id ON iap_purchases(subscription_id);
CREATE INDEX idx_iap_purchases_package_id ON iap_purchases(package_id);
CREATE INDEX idx_iap_purchases_product_id ON iap_purchases(product_id);
CREATE INDEX idx_iap_purchases_transaction_id ON iap_purchases(transaction_id);
CREATE INDEX idx_iap_purchases_order_id ON iap_purchases(order_id);
CREATE INDEX idx_iap_purchases_status ON iap_purchases(status);
CREATE INDEX idx_iap_purchases_expiry_date ON iap_purchases(expiry_date);

-- Create iap_webhook_events table
CREATE TABLE IF NOT EXISTS iap_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store VARCHAR(20) NOT NULL CHECK (store IN ('google_play', 'app_store')),
    event_type VARCHAR(100) NOT NULL,
    purchase_id UUID REFERENCES iap_purchases(id) ON DELETE SET NULL,
    payload JSONB NOT NULL,
    processed_at TIMESTAMP,
    success BOOLEAN DEFAULT false,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for iap_webhook_events
CREATE INDEX idx_iap_webhook_events_store ON iap_webhook_events(store);
CREATE INDEX idx_iap_webhook_events_event_type ON iap_webhook_events(event_type);
CREATE INDEX idx_iap_webhook_events_purchase_id ON iap_webhook_events(purchase_id);
CREATE INDEX idx_iap_webhook_events_created_at ON iap_webhook_events(created_at);
CREATE INDEX idx_iap_webhook_events_success ON iap_webhook_events(success);

-- Add product_id field to subscription_packages if not exists (for mapping IAP products)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'subscription_packages' 
        AND column_name = 'google_play_product_id'
    ) THEN
        ALTER TABLE subscription_packages ADD COLUMN google_play_product_id VARCHAR(255);
        CREATE INDEX idx_packages_google_play_product_id ON subscription_packages(google_play_product_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'subscription_packages' 
        AND column_name = 'app_store_product_id'
    ) THEN
        ALTER TABLE subscription_packages ADD COLUMN app_store_product_id VARCHAR(255);
        CREATE INDEX idx_packages_app_store_product_id ON subscription_packages(app_store_product_id);
    END IF;
END$$;

COMMENT ON TABLE iap_purchases IS 'Stores validated in-app purchase transactions from Google Play and App Store';
COMMENT ON TABLE iap_webhook_events IS 'Logs webhook events from Google Play and App Store for subscription renewals and status changes';
COMMENT ON COLUMN subscription_packages.google_play_product_id IS 'Google Play product/subscription ID for this package';
COMMENT ON COLUMN subscription_packages.app_store_product_id IS 'App Store product ID for this package';
