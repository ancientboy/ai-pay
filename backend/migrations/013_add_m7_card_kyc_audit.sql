-- M7: sandbox virtual cards + rule-based risk + party KYC placeholder + risk audit trail

CREATE TABLE IF NOT EXISTS virtual_card (
  card_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL,
  va_account_id VARCHAR(64) NOT NULL,
  masked_pan VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL,
  credit_limit DECIMAL(24, 8) NOT NULL DEFAULT 0,
  sandbox_reference VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_vc_agent (agent_did),
  CONSTRAINT fk_vc_va FOREIGN KEY (va_account_id) REFERENCES asset_va_account(va_account_id)
);

CREATE TABLE IF NOT EXISTS risk_party_kyc (
  agent_did VARCHAR(128) PRIMARY KEY,
  status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
  tier VARCHAR(32) NOT NULL DEFAULT 'SANDBOX',
  external_reference VARCHAR(128) NOT NULL DEFAULT '',
  verified_at DATETIME NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_kyc_status (status)
);

CREATE TABLE IF NOT EXISTS risk_audit_entry (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  category VARCHAR(64) NOT NULL,
  agent_did VARCHAR(128) NOT NULL DEFAULT '',
  merchant_id VARCHAR(128) NOT NULL DEFAULT '',
  transaction_id VARCHAR(64) NOT NULL DEFAULT '',
  detail_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_ra_agent_created (agent_did, created_at),
  INDEX idx_ra_category (category)
);
