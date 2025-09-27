-- 為 users.user_name 建索引（存在性檢查避免重複建立）
SET @idx_exists := (
  SELECT COUNT(1)
  FROM INFORMATION_SCHEMA.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'users'
    AND INDEX_NAME = 'idx_users_user_name'
);
SET @sql := IF(@idx_exists = 0,
  'CREATE INDEX idx_users_user_name ON users(user_name);',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

