-- Product Catalog Service Database Schema for Google Cloud Spanner

-- Products table: stores product information with pricing
CREATE TABLE products (
    product_id STRING(36) NOT NULL,
    name STRING(255) NOT NULL,
    description STRING(MAX),
    category STRING(100) NOT NULL,
    base_price_numerator INT64 NOT NULL,
    base_price_denominator INT64 NOT NULL,
    discount_percent INT64,
    discount_start_date TIMESTAMP,
    discount_end_date TIMESTAMP,
    status STRING(20) NOT NULL,
    created_at TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
    updated_at TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
    archived_at TIMESTAMP,
) PRIMARY KEY (product_id);

-- Outbox events table: implements transactional outbox pattern
CREATE TABLE outbox_events (
    event_id STRING(36) NOT NULL,
    event_type STRING(100) NOT NULL,
    aggregate_id STRING(36) NOT NULL,
    payload JSON NOT NULL,
    status STRING(20) NOT NULL,
    created_at TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true),
    processed_at TIMESTAMP,
) PRIMARY KEY (event_id);

-- Index for efficient outbox processing
CREATE INDEX idx_outbox_status ON outbox_events(status, created_at);

-- Index for product queries by category
CREATE INDEX idx_products_category ON products(category, status);

-- Index for product queries by status
CREATE INDEX idx_products_status ON products(status, created_at);
