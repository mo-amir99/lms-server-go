-- Initialize database with required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create indexes for better performance (run after migrations)
-- These are examples - adjust based on your actual schema

-- Users table indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_user_type ON users(user_type);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- Subscriptions table indexes
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_package_id ON subscriptions(subscription_package_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_expires_at ON subscriptions(expires_at);

-- Courses table indexes
CREATE INDEX IF NOT EXISTS idx_courses_owner_id ON courses(owner_id);
CREATE INDEX IF NOT EXISTS idx_courses_created_at ON courses(created_at);

-- Payments table indexes
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at);

-- Add any other indexes your queries need
