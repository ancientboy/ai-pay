ALTER TABLE pay_order
  ADD COLUMN fee DECIMAL(24, 8) NOT NULL DEFAULT 0 AFTER amount,
  ADD COLUMN net_amount DECIMAL(24, 8) NOT NULL DEFAULT 0 AFTER fee;

CREATE TABLE IF NOT EXISTS fund_recharge_order (
  recharge_id VARCHAR(64) PRIMARY KEY,
  va_account_id VARCHAR(64) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_recharge_va_created (va_account_id, created_at),
  CONSTRAINT fk_recharge_va FOREIGN KEY (va_account_id) REFERENCES asset_va_account(va_account_id)
);
