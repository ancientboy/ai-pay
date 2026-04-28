CREATE TABLE IF NOT EXISTS provider_account_binding (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  platform_va_account_id VARCHAR(64) NOT NULL,
  agent_did VARCHAR(128) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  provider_customer_id VARCHAR(128) NOT NULL DEFAULT '',
  provider_account_id VARCHAR(128) NOT NULL,
  account_type VARCHAR(32) NOT NULL DEFAULT 'VA',
  currency VARCHAR(8) NOT NULL DEFAULT 'GUSD',
  status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
  metadata_json JSON NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_provider_binding_provider_account (provider, provider_account_id),
  KEY idx_provider_binding_va (platform_va_account_id),
  KEY idx_provider_binding_agent (agent_did)
);

CREATE TABLE IF NOT EXISTS payment_intent (
  intent_id VARCHAR(64) PRIMARY KEY,
  platform_va_account_id VARCHAR(64) NOT NULL,
  agent_did VARCHAR(128) NOT NULL,
  merchant_id VARCHAR(64) NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'GUSD',
  amount DECIMAL(24, 8) NOT NULL,
  route_provider VARCHAR(32) NOT NULL DEFAULT 'local',
  provider_account_id VARCHAR(128) NOT NULL DEFAULT '',
  idem_key VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'CREATED',
  error_message TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_payment_intent_idem (idem_key),
  KEY idx_payment_intent_agent (agent_did),
  KEY idx_payment_intent_va (platform_va_account_id)
);

CREATE TABLE IF NOT EXISTS payment_execution (
  execution_id VARCHAR(64) PRIMARY KEY,
  intent_id VARCHAR(64) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  provider_transaction_id VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'PROCESSING',
  error_message TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_payment_execution_intent (intent_id)
);
