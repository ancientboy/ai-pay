CREATE TABLE IF NOT EXISTS fund_transfer_order (
  transfer_id VARCHAR(64) PRIMARY KEY,
  from_va_account_id VARCHAR(64) NOT NULL,
  to_va_account_id VARCHAR(64) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  status VARCHAR(32) NOT NULL,
  idem_key VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_fund_transfer_idem (idem_key),
  INDEX idx_fund_transfer_from_created (from_va_account_id, created_at),
  INDEX idx_fund_transfer_to_created (to_va_account_id, created_at),
  CONSTRAINT fk_fund_transfer_from_va FOREIGN KEY (from_va_account_id) REFERENCES asset_va_account(va_account_id),
  CONSTRAINT fk_fund_transfer_to_va FOREIGN KEY (to_va_account_id) REFERENCES asset_va_account(va_account_id)
);

CREATE TABLE IF NOT EXISTS fund_withdraw_order (
  withdraw_id VARCHAR(64) PRIMARY KEY,
  va_account_id VARCHAR(64) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  rail VARCHAR(32) NOT NULL,
  destination_hint VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL,
  idem_key VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_fund_withdraw_idem (idem_key),
  INDEX idx_fund_withdraw_va_created (va_account_id, created_at),
  CONSTRAINT fk_fund_withdraw_va FOREIGN KEY (va_account_id) REFERENCES asset_va_account(va_account_id)
);

CREATE TABLE IF NOT EXISTS payment_debit_preview (
  preview_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL,
  merchant_id VARCHAR(128) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  fee_rate DECIMAL(18, 10) NOT NULL,
  fee_amount DECIMAL(24, 8) NOT NULL,
  net_to_merchant DECIMAL(24, 8) NOT NULL,
  fx_rate DECIMAL(24, 8) NOT NULL DEFAULT 1,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_debit_preview_expires (expires_at)
);

CREATE TABLE IF NOT EXISTS x402_outbound_transfer (
  transfer_id VARCHAR(64) PRIMARY KEY,
  va_account_id VARCHAR(64) NOT NULL,
  to_address VARCHAR(256) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  reference_transaction_id VARCHAR(64) NULL,
  status VARCHAR(32) NOT NULL,
  idem_key VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_x402_out_idem (idem_key),
  INDEX idx_x402_out_va_created (va_account_id, created_at),
  CONSTRAINT fk_x402_out_va FOREIGN KEY (va_account_id) REFERENCES asset_va_account(va_account_id)
);
