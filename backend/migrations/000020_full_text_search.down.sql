DROP INDEX IF EXISTS idx_products_sku_trgm;
DROP INDEX IF EXISTS idx_products_name_trgm;
DROP INDEX IF EXISTS idx_products_search_vector;
DROP TRIGGER IF EXISTS products_search_vector_update ON products;
DROP FUNCTION IF EXISTS update_product_search_vector(); 
ALTER TABLE products DROP COLUMN IF EXISTS search_vector;
