CREATE TABLE IF NOT EXISTS webhook_delivery_task (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  webhook_id VARCHAR(64) NOT NULL,
  url VARCHAR(512) NOT NULL,
  event VARCHAR(128) NOT NULL,
  dedupe_key VARCHAR(128) NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5,
  next_retry_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_error VARCHAR(255) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_webhook_delivery_dedupe (webhook_id, event, dedupe_key),
  INDEX idx_webhook_delivery_status_retry (status, next_retry_at),
  INDEX idx_webhook_delivery_event_created (event, created_at)
);
