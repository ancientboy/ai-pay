-- Billing: Stripe subscription sessions and current subscription per platform user

CREATE TABLE IF NOT EXISTS billing_checkout_session (
  id VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(128) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  plan_code VARCHAR(64) NOT NULL,
  payment_rail VARCHAR(32) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  amount_minor BIGINT NULL,
  status VARCHAR(32) NOT NULL,
  checkout_url TEXT NOT NULL,
  provider_session_id VARCHAR(128) NULL,
  provider_subscription_id VARCHAR(128) NULL,
  metadata_json JSON NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_bcs_user (user_id),
  INDEX idx_bcs_provider_sess (provider_session_id)
);

CREATE TABLE IF NOT EXISTS billing_subscription (
  user_id VARCHAR(128) PRIMARY KEY,
  plan_code VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  currency VARCHAR(16) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  provider_customer_id VARCHAR(128) NULL,
  provider_subscription_id VARCHAR(128) NULL,
  current_period_end DATETIME NULL,
  cancel_at_period_end TINYINT(1) NOT NULL DEFAULT 0,
  raw_event_json JSON NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
