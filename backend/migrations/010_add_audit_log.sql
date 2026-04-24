CREATE TABLE IF NOT EXISTS audit_log (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  actor VARCHAR(128) NOT NULL,
  role VARCHAR(32) NOT NULL,
  action VARCHAR(64) NOT NULL,
  resource VARCHAR(255) NOT NULL,
  request_id VARCHAR(128) NOT NULL,
  detail JSON NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_audit_action_created (action, created_at),
  INDEX idx_audit_resource_created (resource, created_at),
  INDEX idx_audit_request_id (request_id)
);
