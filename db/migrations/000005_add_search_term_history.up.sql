ALTER TABLE product_history ADD COLUMN search_term TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_product_history_search_term ON product_history (search_term);
