CREATE TABLE storage_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  document_id BIGINT UNSIGNED NULL, -- sengaja tanpa FK: log bertahan setelah hard delete
  user_id BIGINT UNSIGNED NULL,
  action VARCHAR(40) NOT NULL,
  meta JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_logs_document (document_id),
  KEY idx_logs_user (user_id),
  KEY idx_logs_action_created (action, created_at),
  KEY idx_logs_created (created_at),
  CONSTRAINT fk_logs_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
