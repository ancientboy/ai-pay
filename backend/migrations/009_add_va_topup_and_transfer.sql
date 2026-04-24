CREATE TABLE IF NOT EXISTS va_topup_config (
  va_account_id VARCHAR(64) PRIMARY KEY,
  auto_topup_enabled TINYINT(1) NOT NULL DEFAULT 0,
  threshold_amount DECIMAL(24, 8) NOT NULL DEFAULT 0,
  target_amount DECIMAL(24, 8) NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_topup_va_account FOREIGN KEY (va_account_id) REFERENCES asset_va_account(va_account_id)
);

CREATE TABLE IF NOT EXISTS va_transfer_order (
  transfer_id VARCHAR(64) PRIMARY KEY,
  from_va_account_id VARCHAR(64) NOT NULL,
  to_va_account_id VARCHAR(64) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  status VARCHAR(32) NOT NULL,
  idem_key VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_va_transfer_idem (idem_key),
  INDEX idx_va_transfer_from_created (from_va_account_id, created_at),
  INDEX idx_va_transfer_to_created (to_va_account_id, created_at),
  CONSTRAINT fk_transfer_from_va FOREIGN KEY (from_va_account_id) REFERENCES asset_va_account(va_account_id),
  CONSTRAINT fk_transfer_to_va FOREIGN KEY (to_va_account_id) REFERENCES asset_va_account(va_account_id)
);
