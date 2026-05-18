DROP INDEX IF EXISTS idx_product_history_search_term;
ALTER TABLE product_history DROP COLUMN search_term;
