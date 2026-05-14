CREATE TABLE IF NOT EXISTS product_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_name TEXT NOT NULL,
    price BIGINT NOT NULL,
    link TEXT NOT NULL,
    scraped_at BIGINT NOT NULL
);

CREATE INDEX idx_product_history_name ON product_history (product_name);