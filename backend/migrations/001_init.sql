CREATE TABLE IF NOT EXISTS agent_did (
  did VARCHAR(128) PRIMARY KEY,
  status VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS asset_va_account (
  va_account_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL UNIQUE,
  wallet_address VARCHAR(128) NOT NULL,
  balance DECIMAL(24, 8) NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_asset_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did)
);

CREATE TABLE IF NOT EXISTS pay_authorize_rule (
  agent_did VARCHAR(128) PRIMARY KEY,
  single_limit DECIMAL(24, 8) NOT NULL,
  daily_limit DECIMAL(24, 8) NOT NULL,
  whitelist TEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_rule_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did)
);

CREATE TABLE IF NOT EXISTS pay_order (
  order_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL,
  merchant_id VARCHAR(128) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  status VARCHAR(32) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_pay_order_agent_created (agent_did, created_at),
  CONSTRAINT fk_order_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did)
);
