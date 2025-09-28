-- 支援常見查詢與排序的索引（不變更既有資料內容）
-- products: 搜尋 name 前綴/LIKE、排序 name/value/weight
-- orders: 過濾 user_id、狀態、建立時間，與 orders x products 連接時的 product_id

-- 安全建立索引：先檢查不存在再建立

-- products(name) 前綴/排序
SET @idx_exists := (
  SELECT COUNT(1)
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'products'
    AND INDEX_NAME = 'idx_products_name'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_products_name ON products (name);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- products(value)
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'products' AND INDEX_NAME = 'idx_products_value'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_products_value ON products (value);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- products(weight)
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'products' AND INDEX_NAME = 'idx_products_weight'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_products_weight ON products (weight);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 複合: products(name, product_id) for prefix 範囲 + 安定ソート
SET @idx_exists := (
  SELECT COUNT(1)
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'products'
    AND INDEX_NAME = 'idx_products_name_id'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_products_name_id ON products (name, product_id);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- FULLTEXT: products(name, description) for partial 検索の高速化
SET @idx_exists := (
  SELECT COUNT(1)
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'products'
    AND INDEX_NAME = 'idx_products_fulltext_name_desc'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE FULLTEXT INDEX idx_products_fulltext_name_desc ON products (name, description);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- orders(user_id, created_at) for 履歴一覧の WHERE user_id + ORDER/LIMIT
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_user_created'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_orders_user_created ON orders (user_id, created_at);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- orders(shipped_status, order_id) for robot GetShippingOrders with ORDER BY optimization
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_status_orderid'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_orders_status_orderid ON orders (shipped_status, order_id);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- orders(product_id) for JOIN to products
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'product_id'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_orders_product_id ON orders (product_id);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 複合: orders(user_id, order_id) for 履歴の ORDER BY + LIMIT
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_user_order'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_orders_user_order ON orders (user_id, order_id);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- user_sessions(session_uuid, expires_at) for 認証
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_sessions' AND INDEX_NAME = 'idx_sessions_uuid_expires'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_sessions_uuid_expires ON user_sessions (session_uuid, expires_at);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- products(weight, value, product_id) for robot query optimization
SET @idx_exists := (
  SELECT COUNT(1) FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'products' AND INDEX_NAME = 'idx_products_weight_value_id'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_products_weight_value_id ON products (weight, value, product_id);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


