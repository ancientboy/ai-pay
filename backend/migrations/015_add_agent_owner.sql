ALTER TABLE agent_did
  ADD COLUMN owner_user_id VARCHAR(128) NULL AFTER did;

CREATE INDEX idx_agent_did_owner_user_id ON agent_did(owner_user_id);
