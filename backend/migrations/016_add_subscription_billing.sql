CREATE TABLE IF NOT EXISTS subscription_plan (
  plan_code VARCHAR(32) PRIMARY KEY,
  plan_name VARCHAR(64) NOT NULL,
  monthly_price DECIMAL(12,2) NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'USD',
  max_agents INT NOT NULL,
  api_quota_monthly INT NOT NULL,
  risk_level VARCHAR(32) NOT NULL,
  self_hosted_enabled TINYINT(1) NOT NULL DEFAULT 0,
  support_level VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_subscription (
  user_id VARCHAR(128) PRIMARY KEY,
  plan_code VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL,
  current_period_start TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  current_period_end TIMESTAMP NOT NULL,
  auto_renew TINYINT(1) NOT NULL DEFAULT 1,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_user_subscription_plan FOREIGN KEY (plan_code) REFERENCES subscription_plan(plan_code)
);

CREATE TABLE IF NOT EXISTS subscription_invoice (
  invoice_id VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(128) NOT NULL,
  plan_code VARCHAR(32) NOT NULL,
  amount DECIMAL(12,2) NOT NULL,
  currency VARCHAR(8) NOT NULL DEFAULT 'USD',
  status VARCHAR(32) NOT NULL,
  due_at TIMESTAMP NULL,
  paid_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_subscription_invoice_user_created (user_id, created_at),
  CONSTRAINT fk_subscription_invoice_plan FOREIGN KEY (plan_code) REFERENCES subscription_plan(plan_code)
);

INSERT INTO subscription_plan (
  plan_code, plan_name, monthly_price, currency, max_agents, api_quota_monthly, risk_level, self_hosted_enabled, support_level
)
VALUES
  ('starter', 'Starter', 0.00, 'USD', 20, 100000, 'basic', 0, 'community'),
  ('growth', 'Growth', 299.00, 'USD', 200, 5000000, 'advanced', 1, 'ticket_email'),
  ('enterprise', 'Enterprise', 0.00, 'USD', 999999, 999999999, 'enterprise', 1, 'dedicated')
ON DUPLICATE KEY UPDATE
  plan_name = VALUES(plan_name),
  monthly_price = VALUES(monthly_price),
  currency = VALUES(currency),
  max_agents = VALUES(max_agents),
  api_quota_monthly = VALUES(api_quota_monthly),
  risk_level = VALUES(risk_level),
  self_hosted_enabled = VALUES(self_hosted_enabled),
  support_level = VALUES(support_level);
