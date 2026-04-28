-- M8: self-custody wallet binding, authorization sessions, payment signature workflow

CREATE TABLE IF NOT EXISTS self_host_wallet (
  agent_did VARCHAR(128) PRIMARY KEY,
  wallet_address VARCHAR(256) NOT NULL,
  label VARCHAR(128) NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_shw_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did)
);

CREATE TABLE IF NOT EXISTS auth_session_record (
  session_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_asr_agent (agent_did),
  CONSTRAINT fk_asr_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did)
);

CREATE TABLE IF NOT EXISTS payment_sign_request (
  sign_id VARCHAR(64) PRIMARY KEY,
  agent_did VARCHAR(128) NOT NULL,
  merchant_id VARCHAR(128) NOT NULL,
  amount DECIMAL(24, 8) NOT NULL,
  status VARCHAR(32) NOT NULL,
  session_id VARCHAR(64) NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_psr_agent (agent_did),
  CONSTRAINT fk_psr_agent FOREIGN KEY (agent_did) REFERENCES agent_did(did)
);
