CREATE TABLE IF NOT EXISTS wallet_account (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  agent_did VARCHAR(128) NOT NULL,
  currency VARCHAR(8) NOT NULL,
  chain_id VARCHAR(64) NOT NULL,
  mode VARCHAR(32) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  provider_account_id VARCHAR(128) NOT NULL,
  address VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_wallet_account_agent_currency_chain_mode (agent_did, currency, chain_id, mode),
  KEY idx_wallet_account_provider (provider, provider_account_id)
);
