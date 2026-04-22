ALTER TABLE pay_order
  ADD COLUMN hold_id VARCHAR(64) NULL AFTER net_amount;

CREATE INDEX idx_pay_order_status_created ON pay_order (status, created_at);
CREATE INDEX idx_pay_order_hold_id ON pay_order (hold_id);
