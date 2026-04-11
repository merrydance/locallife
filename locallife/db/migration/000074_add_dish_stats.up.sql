ALTER TABLE dishes ADD COLUMN monthly_sales INT NOT NULL DEFAULT 0;
ALTER TABLE dishes ADD COLUMN repurchase_rate DECIMAL(5,4) NOT NULL DEFAULT 0;

-- Optional: Index for sorting if needed, though composite index interactions can be complex
-- For now, relying on existing indexes + sorting.
-- If performance issues arise, we might add:
-- CREATE INDEX idx_dishes_sort_metrics ON dishes (is_online, monthly_sales DESC, repurchase_rate DESC);
