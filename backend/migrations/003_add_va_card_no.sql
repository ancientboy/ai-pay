ALTER TABLE asset_va_account
  ADD COLUMN va_card_no VARCHAR(32) NULL AFTER va_account_id;

UPDATE asset_va_account
SET va_card_no = CONCAT('vcard_', SUBSTRING(MD5(va_account_id), 1, 16))
WHERE va_card_no IS NULL OR va_card_no = '';

ALTER TABLE asset_va_account
  MODIFY COLUMN va_card_no VARCHAR(32) NOT NULL;

CREATE UNIQUE INDEX idx_asset_va_card_no ON asset_va_account (va_card_no);
