ALTER TABLE asset_va_account
  ADD COLUMN frozen_balance DECIMAL(24, 8) NOT NULL DEFAULT 0 AFTER balance;

CREATE TABLE IF NOT EXISTS account_hold (
  hold_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL,
  va_account_id VARCHAR(64) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_hold_agent_created (agent_did, created_at),
  INDEX idx_hold_va_status (va_account_id, status),
  CONSTRAINT fk_hold_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did),
  CONSTRAINT fk_hold_va FOREIGN KEY (va_account_id) REFERENCES asset_va_account(va_account_id)
);
