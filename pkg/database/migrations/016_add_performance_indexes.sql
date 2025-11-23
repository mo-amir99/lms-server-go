-- Migration: Add performance indexes for frequently queried columns
-- This migration adds indexes to optimize slow queries identified in production

-- Users table indexes
CREATE INDEX IF NOT EXISTS idx_users_email ON users(LOWER(email));
CREATE INDEX IF NOT EXISTS idx_users_subscription_id ON users(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_user_type ON users(user_type) WHERE user_type IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
CREATE INDEX IF NOT EXISTS idx_users_subscription_type_active ON users(subscription_id, user_type, is_active) WHERE subscription_id IS NOT NULL AND user_type = 'student';

-- Subscriptions table indexes (id is already primary key, but adding partial indexes)
-- Note: user_id already has an index from GORM tag, but adding composite index for better performance
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_active ON subscriptions(user_id, is_active);

-- Courses table indexes
CREATE INDEX IF NOT EXISTS idx_courses_subscription_id ON courses(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_courses_is_active ON courses(is_active);
CREATE INDEX IF NOT EXISTS idx_courses_subscription_order ON courses(subscription_id, "order", name) WHERE is_active = true;

-- Lessons table indexes
CREATE INDEX IF NOT EXISTS idx_lessons_course_id ON lessons(course_id) WHERE course_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_lessons_course_order ON lessons(course_id, "order", name);

-- Attachments table indexes
CREATE INDEX IF NOT EXISTS idx_attachments_lesson_id ON attachments(lesson_id) WHERE lesson_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_attachments_lesson_order ON attachments(lesson_id, "order", name) WHERE is_active = true;

-- Group access table indexes
CREATE INDEX IF NOT EXISTS idx_group_access_subscription_id ON group_access(subscription_id) WHERE subscription_id IS NOT NULL;

-- Announcements table indexes
CREATE INDEX IF NOT EXISTS idx_announcements_subscription_id ON announcements(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_announcements_subscription_active_created ON announcements(subscription_id, created_at DESC) WHERE is_active = true;

-- Comments table indexes (if exists)
CREATE INDEX IF NOT EXISTS idx_comments_lesson_id ON comments(lesson_id) WHERE lesson_id IS NOT NULL;

-- Payments table indexes (if exists)
-- Note: payments table uses subscription_id, not user_id
CREATE INDEX IF NOT EXISTS idx_payments_subscription_date ON payments(subscription_id, date DESC);

-- Forums table indexes
CREATE INDEX IF NOT EXISTS idx_forums_subscription_id ON forums(subscription_id) WHERE subscription_id IS NOT NULL;

-- Threads table indexes
CREATE INDEX IF NOT EXISTS idx_threads_forum_id ON threads(forum_id) WHERE forum_id IS NOT NULL;

-- Support tickets table indexes
CREATE INDEX IF NOT EXISTS idx_support_tickets_subscription_id ON support_tickets(subscription_id) WHERE subscription_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_support_tickets_user_id ON support_tickets(user_id) WHERE user_id IS NOT NULL;

-- Composite index for dashboard queries
CREATE INDEX IF NOT EXISTS idx_lessons_course_subscription ON lessons(course_id) INCLUDE (id);
CREATE INDEX IF NOT EXISTS idx_courses_subscription_stats ON courses(subscription_id) INCLUDE (storage_usage_in_gb, file_storage_gb, stream_storage_gb);
