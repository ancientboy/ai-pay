package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"ai-pay-backend/internal/storage"
	"github.com/redis/go-redis/v9"
)

type PersistentService struct {
	store *storage.Store
}

type scanRows interface {
	Scan(dest ...any) error
}

func NewPersistent(store *storage.Store) *PersistentService {
	return &PersistentService{store: store}
}

func (s *PersistentService) RegisterAgent(did string) Agent {
	_, _ = s.store.DB.Exec(`INSERT IGNORE INTO agent_did (did, status, created_at) VALUES (?, 'ACTIVE', UTC_TIMESTAMP())`, did)
	return Agent{DID: did}
}

func (s *PersistentService) SetAgentPublicKey(did string, pubKey string) error {
	if _, err := s.store.DB.Exec(`UPDATE agent_did SET did_pub_key = ? WHERE did = ?`, pubKey, did); err != nil {
		return err
	}
	return nil
}

func (s *PersistentService) AgentPublicKey(did string) (string, error) {
	var pub string
	if err := s.store.DB.QueryRow(`SELECT COALESCE(did_pub_key, '') FROM agent_did WHERE did = ?`, did).Scan(&pub); err != nil {
		return "", err
	}
	return pub, nil
}

func (s *PersistentService) VerifyAgentSignature(did string, message string, signature string) error {
	pub, err := s.AgentPublicKey(did)
	if err != nil {
		return err
	}
	if strings.TrimSpace(pub) == "" {
		return fmt.Errorf("did public key missing")
	}
	if !verifySignature(pub, signature, []byte(message)) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func (s *PersistentService) UpdateAgentPublicKey(did string, newPubKey string, proofMessage string, proofSignature string) error {
	if strings.TrimSpace(did) == "" || strings.TrimSpace(newPubKey) == "" {
		return fmt.Errorf("invalid request")
	}
	if err := s.VerifyAgentSignature(did, proofMessage, proofSignature); err != nil {
		return err
	}
	return s.SetAgentPublicKey(did, newPubKey)
}

func (s *PersistentService) CreateAccount(agentDID string) Account {
	wallet := fmt.Sprintf("0xwallet_%d", time.Now().UnixNano())
	va := fmt.Sprintf("va_%d", time.Now().UnixNano())
	cardNo := fmt.Sprintf("6888%012d", time.Now().UnixNano()%1000000000000)
	_, _ = s.store.DB.Exec(`
INSERT INTO asset_va_account (va_account_id, va_card_no, agent_did, wallet_address, balance, status, created_at)
VALUES (?, ?, ?, ?, 0, 'ACTIVE', UTC_TIMESTAMP())`, va, cardNo, agentDID, wallet)
	return Account{WalletAddress: wallet, VAAccountID: va, VACardNo: cardNo, AgentDID: agentDID, Balance: 0, FrozenBalance: 0}
}

func (s *PersistentService) Recharge(va string, amount string, idemKey string) error {
	amountV, err := parseAmount(amount)
	if err != nil || amountV <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	if idemKey == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	ctx := context.Background()
	rechargeIdemKey := "idem:rch:" + idemKey
	if found, err := s.store.Redis.Get(ctx, rechargeIdemKey).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return &APIError{Code: "PAY-010", Message: "idempotency check failed"}
	}
	rechargeID := fmt.Sprintf("rch_%d", time.Now().UnixNano())
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "begin tx failed"}
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE asset_va_account SET balance = balance + ? WHERE va_account_id = ? OR va_card_no = ?`, amountV, va, va)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "recharge failed"}
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	var resolvedVA string
	if err := tx.QueryRow(`SELECT va_account_id FROM asset_va_account WHERE va_account_id = ? OR va_card_no = ? LIMIT 1`, va, va).Scan(&resolvedVA); err != nil {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	_, err = tx.Exec(`INSERT INTO fund_recharge_order (recharge_id, va_account_id, amount, status, created_at) VALUES (?, ?, ?, 'SETTLED', UTC_TIMESTAMP())`,
		rechargeID, resolvedVA, amountV)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "recharge log failed"}
	}
	if err := tx.Commit(); err != nil {
		return &APIError{Code: "PAY-010", Message: "recharge commit failed"}
	}
	if err := s.store.Redis.Set(ctx, rechargeIdemKey, rechargeID, 24*time.Hour).Err(); err != nil {
		return &APIError{Code: "PAY-010", Message: "idempotency write failed"}
	}
	return nil
}

func (s *PersistentService) SetAuthorizeRule(agentDID string, single string, daily string, merchants []string) error {
	singleV, err := parseAmount(single)
	if err != nil {
		return err
	}
	dailyV, err := parseAmount(daily)
	if err != nil {
		return err
	}
	white := ""
	for i, m := range merchants {
		if i > 0 {
			white += ","
		}
		white += m
	}
	_, err = s.store.DB.Exec(`
INSERT INTO pay_authorize_rule (agent_did, single_limit, daily_limit, whitelist, status, updated_at)
VALUES (?, ?, ?, ?, 'ACTIVE', UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE single_limit = VALUES(single_limit), daily_limit = VALUES(daily_limit), whitelist = VALUES(whitelist), updated_at = UTC_TIMESTAMP()`,
		agentDID, singleV, dailyV, white)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "set authorize failed"}
	}
	return nil
}

func (s *PersistentService) UpdateAuthorizeRule(agentDID string, single string, daily string, merchants []string) error {
	singleV, err := parseAmount(single)
	if err != nil {
		return err
	}
	dailyV, err := parseAmount(daily)
	if err != nil {
		return err
	}
	white := ""
	for i, m := range merchants {
		if i > 0 {
			white += ","
		}
		white += m
	}
	result, err := s.store.DB.Exec(`
UPDATE pay_authorize_rule
SET single_limit = ?, daily_limit = ?, whitelist = ?, updated_at = UTC_TIMESTAMP()
WHERE agent_did = ?`, singleV, dailyV, white, agentDID)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "update authorize failed"}
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		var exists int
		if err := s.store.DB.QueryRow(`SELECT COUNT(1) FROM pay_authorize_rule WHERE agent_did = ?`, agentDID).Scan(&exists); err != nil {
			return &APIError{Code: "PAY-010", Message: "update authorize failed"}
		}
		if exists == 0 {
			return &APIError{Code: "PAY-002", Message: "missing authorization rule"}
		}
	}
	return nil
}

func (s *PersistentService) FreezeAuthorizeRule(agentDID string) error {
	result, err := s.store.DB.Exec(`
UPDATE pay_authorize_rule
SET status = 'FROZEN', updated_at = UTC_TIMESTAMP()
WHERE agent_did = ?`, agentDID)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "freeze authorize failed"}
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	return nil
}

func (s *PersistentService) ActivateAuthorizeRule(agentDID string) error {
	result, err := s.store.DB.Exec(`
UPDATE pay_authorize_rule
SET status = 'ACTIVE', updated_at = UTC_TIMESTAMP()
WHERE agent_did = ?`, agentDID)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "activate authorize failed"}
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	return nil
}

func (s *PersistentService) QueryInterest(accountID string) (InterestQuote, error) {
	accountID = strings.TrimSpace(accountID)
	var balance float64
	var createdAt time.Time
	if err := s.store.DB.QueryRow(`
SELECT balance, created_at
FROM asset_va_account
WHERE va_account_id = ?`, accountID).Scan(&balance, &createdAt); err != nil {
		return InterestQuote{}, err
	}
	now := time.Now().UTC()
	days := now.Sub(createdAt.UTC()).Hours() / 24
	if days < 0 {
		days = 0
	}
	const annualRate = 0.03
	accrued := balance * annualRate * (days / 365.0)
	return InterestQuote{
		AccountID:       accountID,
		AnnualRate:      annualRate,
		AccruedInterest: accrued,
		AsOf:            now,
	}, nil
}

func (s *PersistentService) SetVATopupConfig(accountID string, autoTopup bool, threshold string, target string) (VATopupConfig, error) {
	thresholdV, err := parseAmount(threshold)
	if err != nil || thresholdV < 0 {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "invalid threshold amount"}
	}
	targetV, err := parseAmount(target)
	if err != nil || targetV <= 0 || targetV < thresholdV {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "invalid target amount"}
	}
	accountID = strings.TrimSpace(accountID)
	exists := 0
	if err := s.store.DB.QueryRow(`SELECT COUNT(1) FROM asset_va_account WHERE va_account_id = ?`, accountID).Scan(&exists); err != nil {
		return VATopupConfig{}, err
	}
	if exists == 0 {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if _, err := s.store.DB.Exec(`
INSERT INTO va_topup_config (va_account_id, auto_topup_enabled, threshold_amount, target_amount, updated_at)
VALUES (?, ?, ?, ?, UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE
  auto_topup_enabled = VALUES(auto_topup_enabled),
  threshold_amount = VALUES(threshold_amount),
  target_amount = VALUES(target_amount),
  updated_at = UTC_TIMESTAMP()`, accountID, autoTopup, thresholdV, targetV); err != nil {
		return VATopupConfig{}, err
	}
	return s.GetVATopupConfig(accountID)
}

func (s *PersistentService) GetVATopupConfig(accountID string) (VATopupConfig, error) {
	accountID = strings.TrimSpace(accountID)
	var item VATopupConfig
	if err := s.store.DB.QueryRow(`
SELECT va_account_id, auto_topup_enabled, threshold_amount, target_amount, updated_at
FROM va_topup_config
WHERE va_account_id = ?`, accountID).Scan(
		&item.AccountID,
		&item.AutoTopupEnabled,
		&item.ThresholdAmount,
		&item.TargetAmount,
		&item.UpdatedAt,
	); err == nil {
		return item, nil
	} else if err != sql.ErrNoRows {
		return VATopupConfig{}, err
	}
	exists := 0
	if err := s.store.DB.QueryRow(`SELECT COUNT(1) FROM asset_va_account WHERE va_account_id = ?`, accountID).Scan(&exists); err != nil {
		return VATopupConfig{}, err
	}
	if exists == 0 {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	return VATopupConfig{
		AccountID:        accountID,
		AutoTopupEnabled: false,
		ThresholdAmount:  0,
		TargetAmount:     0,
		UpdatedAt:        time.Now().UTC(),
	}, nil
}

func (s *PersistentService) TransferVA(fromAccountID string, toAccountID string, amount string, idemKey string) error {
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	amountV, err := parseAmount(amount)
	if err != nil || amountV <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	fromID := strings.TrimSpace(fromAccountID)
	toID := strings.TrimSpace(toAccountID)
	if fromID == "" || toID == "" || fromID == toID {
		return &APIError{Code: "PAY-010", Message: "invalid account id"}
	}
	ctx := context.Background()
	idem := "idem:va_transfer:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return err
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
UPDATE asset_va_account
SET balance = balance - ?
WHERE va_account_id = ? AND balance >= ?`, amountV, fromID, amountV)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	res, err = tx.Exec(`
UPDATE asset_va_account
SET balance = balance + ?
WHERE va_account_id = ?`, amountV, toID)
	if err != nil {
		return err
	}
	affected, _ = res.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	transferID := fmt.Sprintf("vat_%d", time.Now().UnixNano())
	if _, err := tx.Exec(`
INSERT INTO va_transfer_order (transfer_id, from_va_account_id, to_va_account_id, amount, status, idem_key, created_at)
VALUES (?, ?, ?, ?, 'SETTLED', ?, UTC_TIMESTAMP())`, transferID, fromID, toID, amountV, idemKey); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.store.Redis.Set(ctx, idem, transferID, 24*time.Hour).Err()
}

func (s *PersistentService) ListVATransfers(accountID string, status string, startTime string, endTime string, limit int, offset int) []VATransferRecord {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := `
SELECT transfer_id, from_va_account_id, to_va_account_id, amount, status, idem_key, created_at
FROM va_transfer_order
WHERE 1=1`
	args := make([]any, 0, 4)
	accountID = strings.TrimSpace(accountID)
	status = strings.ToUpper(strings.TrimSpace(status))
	if accountID != "" {
		query += " AND (from_va_account_id = ? OR to_va_account_id = ?)"
		args = append(args, accountID, accountID)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if trimmed := strings.TrimSpace(startTime); trimmed != "" {
		if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
			query += " AND created_at >= ?"
			args = append(args, t.UTC())
		}
	}
	if trimmed := strings.TrimSpace(endTime); trimmed != "" {
		if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
			query += " AND created_at <= ?"
			args = append(args, t.UTC())
		}
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.store.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]VATransferRecord, 0, limit)
	for rows.Next() {
		var item VATransferRecord
		if err := rows.Scan(
			&item.TransferID,
			&item.FromAccountID,
			&item.ToAccountID,
			&item.Amount,
			&item.Status,
			&item.IdempotencyKey,
			&item.CreatedAt,
		); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) AppendAuditLog(actor string, role string, action string, resource string, requestID string, detail map[string]any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = s.store.DB.Exec(`
INSERT INTO audit_log (actor, role, action, resource, request_id, detail, created_at)
VALUES (?, ?, ?, ?, ?, ?, UTC_TIMESTAMP())`,
		strings.TrimSpace(actor),
		strings.TrimSpace(role),
		strings.TrimSpace(action),
		strings.TrimSpace(resource),
		strings.TrimSpace(requestID),
		string(raw),
	)
	return err
}

func (s *PersistentService) ListAuditLogs(action string, resource string, limit int, offset int) []AuditLog {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query := `
SELECT id, actor, role, action, resource, request_id, detail, created_at
FROM audit_log
WHERE 1=1`
	args := make([]any, 0, 4)
	if trimmed := strings.TrimSpace(action); trimmed != "" {
		query += " AND action = ?"
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(resource); trimmed != "" {
		query += " AND resource = ?"
		args = append(args, trimmed)
	}
	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.store.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]AuditLog, 0, limit)
	for rows.Next() {
		var item AuditLog
		var detail string
		if err := rows.Scan(&item.ID, &item.Actor, &item.Role, &item.Action, &item.Resource, &item.RequestID, &detail, &item.CreatedAt); err != nil {
			continue
		}
		item.Detail = json.RawMessage([]byte(detail))
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) GetRiskConfig() RiskConfig {
	cfg, err := s.getRiskConfig()
	if err != nil {
		return RiskConfig{
			Enabled:           true,
			SingleAmountLimit: 1000,
			BlockedMerchants:  []string{"m_risk_block"},
			UpdatedAt:         time.Now().UTC(),
		}
	}
	return cfg
}

func (s *PersistentService) SetRiskConfig(enabled bool, singleAmountLimit string, blockedMerchants []string) (RiskConfig, error) {
	limit, err := parseAmount(singleAmountLimit)
	if err != nil || limit <= 0 {
		return RiskConfig{}, &APIError{Code: "PAY-010", Message: "invalid singleAmountLimit"}
	}
	trimmed := make([]string, 0, len(blockedMerchants))
	seen := map[string]struct{}{}
	for _, item := range blockedMerchants {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		trimmed = append(trimmed, v)
	}
	raw, _ := json.Marshal(trimmed)
	if _, err := s.store.DB.Exec(`
INSERT INTO risk_config (id, enabled, single_amount_limit, blocked_merchants, updated_at)
VALUES (1, ?, ?, ?, UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE
  enabled = VALUES(enabled),
  single_amount_limit = VALUES(single_amount_limit),
  blocked_merchants = VALUES(blocked_merchants),
  updated_at = UTC_TIMESTAMP()`, enabled, limit, string(raw)); err != nil {
		return RiskConfig{}, err
	}
	return s.getRiskConfig()
}

func (s *PersistentService) ListChannelRoutes() []ChannelRoute {
	routes := s.loadChannelRoutes()
	out := make([]ChannelRoute, 0, len(routes))
	for _, item := range routes {
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) SetChannelRoute(merchantID string, mode string) (ChannelRoute, error) {
	merchantID = strings.TrimSpace(merchantID)
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if merchantID == "" || !isValidChannelMode(mode) {
		return ChannelRoute{}, &APIError{Code: "PAY-010", Message: "invalid channel route"}
	}
	if _, err := s.store.DB.Exec(`
INSERT INTO channel_route (merchant_id, mode, updated_at)
VALUES (?, ?, UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE
  mode = VALUES(mode),
  updated_at = UTC_TIMESTAMP()`, merchantID, mode); err != nil {
		return ChannelRoute{}, err
	}
	return ChannelRoute{MerchantID: merchantID, Mode: mode, UpdatedAt: time.Now().UTC()}, nil
}

func (s *PersistentService) DeleteChannelRoute(merchantID string) error {
	merchantID = strings.TrimSpace(merchantID)
	if merchantID == "" {
		return &APIError{Code: "PAY-010", Message: "invalid merchantId"}
	}
	_, err := s.store.DB.Exec(`DELETE FROM channel_route WHERE merchant_id = ?`, merchantID)
	return err
}

func (s *PersistentService) Pay(req PayRequest) (PayResponse, *APIError) {
	if req.Signature == "" {
		return PayResponse{}, &APIError{Code: "PAY-001", Message: "signature required"}
	}
	if req.IdempotencyKey == "" {
		return PayResponse{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	amount, err := parseAmount(req.Amount)
	if err != nil || amount <= 0 {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	ctx := context.Background()
	idemKey := "idem:" + req.IdempotencyKey
	foundTxn, err := s.store.Redis.Get(ctx, idemKey).Result()
	if err == nil && foundTxn != "" {
		var status string
		if queryErr := s.store.DB.QueryRow(`SELECT status FROM pay_order WHERE order_id = ?`, foundTxn).Scan(&status); queryErr == nil {
			return PayResponse{TransactionID: foundTxn, Status: status}, nil
		}
		return PayResponse{TransactionID: foundTxn, Status: "SETTLED"}, nil
	}
	if err != nil && err != redis.Nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "idempotency check failed"}
	}

	var single, daily float64
	var whiteList string
	err = s.store.DB.QueryRow(`SELECT single_limit, daily_limit, whitelist FROM pay_authorize_rule WHERE agent_did = ? AND status='ACTIVE'`, req.PayerDID).Scan(&single, &daily, &whiteList)
	if err != nil {
		if err == sql.ErrNoRows {
			return PayResponse{}, &APIError{Code: "PAY-002", Message: "missing authorization rule"}
		}
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "query rule failed"}
	}
	if amount > single {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "single limit exceeded"}
	}
	var todaySpent float64
	if err := s.store.DB.QueryRow(`
SELECT COALESCE(SUM(amount), 0)
FROM pay_order
WHERE agent_did = ?
  AND status = 'SETTLED'
  AND created_at >= UTC_DATE()
  AND created_at < UTC_DATE() + INTERVAL 1 DAY`, req.PayerDID).Scan(&todaySpent); err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "query daily spent failed"}
	}
	if todaySpent+amount > daily {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "daily limit exceeded"}
	}
	if !merchantInWhitelist(whiteList, req.MerchantID) {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "merchant not whitelisted"}
	}
	if riskErr := s.evaluateP2Risk(req.MerchantID, amount); riskErr != nil {
		return PayResponse{}, riskErr
	}

	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "begin tx failed"}
	}
	defer tx.Rollback()

	var vaID string
	err = tx.QueryRow(`SELECT va_account_id FROM asset_va_account WHERE agent_did = ? FOR UPDATE`, req.PayerDID).Scan(&vaID)
	if err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	holdID, apiErr := s.freezeAmountTx(tx, req.PayerDID, vaID, amount)
	if apiErr != nil {
		return PayResponse{}, apiErr
	}
	txnID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	switch s.decideChannelPath(req.MerchantID) {
	case channelFail:
		if err := s.releaseHoldTx(tx, holdID); err != nil {
			return PayResponse{}, &APIError{Code: "PAY-010", Message: "hold release failed"}
		}
		if err := tx.Commit(); err != nil {
			return PayResponse{}, &APIError{Code: "PAY-010", Message: "rollback commit failed"}
		}
		return PayResponse{}, &APIError{Code: "PAY-007", Message: "channel timeout"}
	case channelSettling:
		if _, err := tx.Exec(`INSERT INTO pay_order (order_id, agent_did, merchant_id, amount, fee, net_amount, hold_id, status, created_at) VALUES (?, ?, ?, ?, 0, 0, ?, 'SETTLING', UTC_TIMESTAMP())`,
			txnID, req.PayerDID, req.MerchantID, amount, holdID); err != nil {
			return PayResponse{}, &APIError{Code: "PAY-010", Message: "create settling order failed"}
		}
		if err := tx.Commit(); err != nil {
			return PayResponse{}, &APIError{Code: "PAY-010", Message: "commit failed"}
		}
		if err := s.store.Redis.Set(ctx, idemKey, txnID, 24*time.Hour).Err(); err != nil {
			return PayResponse{}, &APIError{Code: "PAY-010", Message: "idempotency write failed"}
		}
		return PayResponse{TransactionID: txnID, Status: "SETTLING"}, nil
	}
	if err := s.debitHoldTx(tx, holdID); err != nil {
		_ = s.releaseHoldTx(tx, holdID)
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "debit failed"}
	}
	fee := calcFee(amount)
	netAmount := amount - fee
	if _, err := tx.Exec(`INSERT INTO pay_order (order_id, agent_did, merchant_id, amount, fee, net_amount, hold_id, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'SETTLED', UTC_TIMESTAMP())`,
		txnID, req.PayerDID, req.MerchantID, amount, fee, netAmount, holdID); err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "create order failed"}
	}
	if err := tx.Commit(); err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "commit failed"}
	}
	if err := s.store.Redis.Set(ctx, idemKey, txnID, 24*time.Hour).Err(); err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "idempotency write failed"}
	}
	return PayResponse{TransactionID: txnID, Status: "SETTLED"}, nil
}

func (s *PersistentService) freezeAmountTx(tx *sql.Tx, agentDID, vaID string, amount float64) (string, *APIError) {
	result, err := tx.Exec(`
UPDATE asset_va_account
SET balance = balance - ?, frozen_balance = frozen_balance + ?
WHERE va_account_id = ? AND agent_did = ? AND balance >= ?`, amount, amount, vaID, agentDID, amount)
	if err != nil {
		return "", &APIError{Code: "PAY-010", Message: "freeze failed"}
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return "", &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	holdID := fmt.Sprintf("hold_%d", time.Now().UnixNano())
	if _, err := tx.Exec(`
INSERT INTO account_hold (hold_id, agent_did, va_account_id, amount, status, created_at, updated_at)
VALUES (?, ?, ?, ?, 'FROZEN', UTC_TIMESTAMP(), UTC_TIMESTAMP())`, holdID, agentDID, vaID, amount); err != nil {
		return "", &APIError{Code: "PAY-010", Message: "hold create failed"}
	}
	return holdID, nil
}

func (s *PersistentService) debitHoldTx(tx *sql.Tx, holdID string) error {
	var amount float64
	var vaID string
	if err := tx.QueryRow(`
SELECT amount, va_account_id FROM account_hold
WHERE hold_id = ? AND status = 'FROZEN' FOR UPDATE`, holdID).Scan(&amount, &vaID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE asset_va_account
SET frozen_balance = frozen_balance - ?
WHERE va_account_id = ? AND frozen_balance >= ?`, amount, vaID, amount); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE account_hold
SET status = 'DEBITED', updated_at = UTC_TIMESTAMP()
WHERE hold_id = ?`, holdID); err != nil {
		return err
	}
	return nil
}

func (s *PersistentService) releaseHoldTx(tx *sql.Tx, holdID string) error {
	var amount float64
	var vaID string
	if err := tx.QueryRow(`
SELECT amount, va_account_id FROM account_hold
WHERE hold_id = ? AND status = 'FROZEN' FOR UPDATE`, holdID).Scan(&amount, &vaID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE asset_va_account
SET frozen_balance = frozen_balance - ?, balance = balance + ?
WHERE va_account_id = ? AND frozen_balance >= ?`, amount, amount, vaID, amount); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE account_hold
SET status = 'RELEASED', updated_at = UTC_TIMESTAMP()
WHERE hold_id = ?`, holdID); err != nil {
		return err
	}
	return nil
}

func (s *PersistentService) QueryStatus(txID string) (Transaction, error) {
	var tx Transaction
	var amount, fee, netAmount float64
	err := s.store.DB.QueryRow(`SELECT order_id, agent_did, merchant_id, amount, fee, net_amount, status, created_at FROM pay_order WHERE order_id = ?`, txID).
		Scan(&tx.ID, &tx.PayerDID, &tx.Merchant, &amount, &fee, &netAmount, &tx.Status, &tx.CreatedAt)
	if err != nil {
		return Transaction{}, err
	}
	tx.Amount = amount
	tx.Fee = fee
	tx.NetAmount = netAmount
	return tx, nil
}

func (s *PersistentService) ResolveSettling(transactionID string, success bool) error {
	ctx := context.Background()
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var holdID sql.NullString
	var status string
	var amount float64
	if err := tx.QueryRow(`
SELECT hold_id, status, amount
FROM pay_order
WHERE order_id = ? FOR UPDATE`, transactionID).Scan(&holdID, &status, &amount); err != nil {
		return err
	}
	if status != "SETTLING" {
		return fmt.Errorf("transaction not settling")
	}
	if !holdID.Valid || holdID.String == "" {
		return fmt.Errorf("missing hold id")
	}

	if success {
		if err := s.debitHoldTx(tx, holdID.String); err != nil {
			return err
		}
		fee := calcFee(amount)
		net := amount - fee
		if _, err := tx.Exec(`
UPDATE pay_order
SET status = 'SETTLED', fee = ?, net_amount = ?
WHERE order_id = ?`, fee, net, transactionID); err != nil {
			return err
		}
		payload := map[string]any{
			"transactionId": transactionID,
			"status":        "SETTLED",
			"amount":        amount,
			"fee":           fee,
			"netAmount":     net,
			"time":          time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.enqueueWebhookEventTx(tx, "payment.settled", "tx:"+transactionID, payload); err != nil {
			return err
		}
	} else {
		if err := s.releaseHoldTx(tx, holdID.String); err != nil {
			return err
		}
		if _, err := tx.Exec(`
UPDATE pay_order
SET status = 'FAILED'
WHERE order_id = ?`, transactionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PersistentService) Unfreeze(transactionID string, idemKey string) error {
	if idemKey == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	ctx := context.Background()
	idem := "idem:unfreeze:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return err
	}
	if err := s.ResolveSettling(transactionID, false); err != nil {
		return err
	}
	return s.store.Redis.Set(ctx, idem, transactionID, 24*time.Hour).Err()
}

func (s *PersistentService) Refund(transactionID string, idemKey string) error {
	if idemKey == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	ctx := context.Background()
	idem := "idem:refund:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return err
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var agentDID string
	var amount float64
	var status string
	if err := tx.QueryRow(`
SELECT agent_did, amount, status
FROM pay_order
WHERE order_id = ? FOR UPDATE`, transactionID).Scan(&agentDID, &amount, &status); err != nil {
		return err
	}
	if status != "SETTLED" {
		return fmt.Errorf("transaction not settled")
	}
	if _, err := tx.Exec(`
UPDATE asset_va_account
SET balance = balance + ?
WHERE agent_did = ?`, amount, agentDID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE pay_order
SET status = 'REFUNDED'
WHERE order_id = ?`, transactionID); err != nil {
		return err
	}
	payload := map[string]any{
		"transactionId": transactionID,
		"status":        "REFUNDED",
		"amount":        amount,
		"agentDid":      agentDID,
		"time":          time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.enqueueWebhookEventTx(tx, "payment.refunded", "tx:"+transactionID, payload); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.store.Redis.Set(ctx, idem, transactionID, 24*time.Hour).Err()
}

func (s *PersistentService) enqueueWebhookEventTx(tx *sql.Tx, event, dedupeKey string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	rows, err := tx.Query(`
SELECT id, url
FROM developer_webhook
WHERE event = ?`, event)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var webhookID, url string
		if err := rows.Scan(&webhookID, &url); err != nil {
			continue
		}
		if _, err := tx.Exec(`
INSERT IGNORE INTO webhook_delivery_task
  (webhook_id, url, event, dedupe_key, payload, status, attempts, max_attempts, next_retry_at, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, 'PENDING', 0, 5, UTC_TIMESTAMP(), UTC_TIMESTAMP(), UTC_TIMESTAMP())`,
			webhookID, url, event, dedupeKey, string(body)); err != nil {
			return err
		}
	}
	return nil
}

func (s *PersistentService) BalanceByVA(va string) (float64, error) {
	var balance float64
	err := s.store.DB.QueryRow(`SELECT balance FROM asset_va_account WHERE va_account_id = ?`, va).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *PersistentService) LedgerByVA(va string) []Transaction {
	rows, err := s.store.DB.Query(`
SELECT p.order_id, p.agent_did, p.merchant_id, p.amount, p.fee, p.net_amount, p.status, p.created_at
FROM pay_order p
JOIN asset_va_account a ON p.agent_did = a.agent_did
WHERE a.va_account_id = ?
ORDER BY p.created_at DESC LIMIT 100`, va)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]Transaction, 0)
	for rows.Next() {
		var tx Transaction
		var amount, fee, netAmount float64
		if err := rows.Scan(&tx.ID, &tx.PayerDID, &tx.Merchant, &amount, &fee, &netAmount, &tx.Status, &tx.CreatedAt); err != nil {
			continue
		}
		tx.Amount = amount
		tx.Fee = fee
		tx.NetAmount = netAmount
		out = append(out, tx)
	}
	return out
}

func (s *PersistentService) OverviewMetrics() (OverviewMetrics, error) {
	var totalBalance float64
	if err := s.store.DB.QueryRow(`SELECT COALESCE(SUM(balance + frozen_balance),0) FROM asset_va_account`).Scan(&totalBalance); err != nil {
		return OverviewMetrics{}, err
	}

	var todaySpend float64
	if err := s.store.DB.QueryRow(`
SELECT COALESCE(SUM(amount),0) FROM pay_order
WHERE status='SETTLED'
  AND created_at >= UTC_DATE()
  AND created_at < UTC_DATE() + INTERVAL 1 DAY`).Scan(&todaySpend); err != nil {
		return OverviewMetrics{}, err
	}

	var totalOrders int
	if err := s.store.DB.QueryRow(`SELECT COUNT(*) FROM pay_order`).Scan(&totalOrders); err != nil {
		return OverviewMetrics{}, err
	}

	var successOrders int
	if err := s.store.DB.QueryRow(`SELECT COUNT(*) FROM pay_order WHERE status='SETTLED'`).Scan(&successOrders); err != nil {
		return OverviewMetrics{}, err
	}

	successRate := 100.0
	if totalOrders > 0 {
		successRate = float64(successOrders) / float64(totalOrders) * 100
	}

	return OverviewMetrics{
		TotalBalance:       totalBalance,
		TodaySpend:         todaySpend,
		PaymentSuccessRate: successRate,
		AlertCount:         0,
	}, nil
}

func (s *PersistentService) ListAgents() []AgentSummary {
	rows, err := s.store.DB.Query(`
SELECT agent_did, va_account_id, va_card_no, wallet_address, balance, status
FROM asset_va_account
ORDER BY created_at DESC
LIMIT 200`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]AgentSummary, 0)
	for rows.Next() {
		var item AgentSummary
		if err := rows.Scan(&item.AgentDID, &item.VAAccountID, &item.VACardNo, &item.WalletAddress, &item.Balance, &item.Status); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) ListRecharges(va string, limit int) []RechargeOrder {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	query := `
SELECT recharge_id, va_account_id, amount, status, created_at
FROM fund_recharge_order`
	args := make([]any, 0, 2)
	if va != "" {
		query += " WHERE va_account_id = ?"
		args = append(args, va)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.store.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]RechargeOrder, 0)
	for rows.Next() {
		var item RechargeOrder
		if err := rows.Scan(&item.RechargeID, &item.VAAccountID, &item.Amount, &item.Status, &item.CreatedAt); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) ListDeveloperAPIKeys(limit int) []DeveloperAPIKey {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.store.DB.Query(`
SELECT id, name, api_key, created_at
FROM developer_api_key
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]DeveloperAPIKey, 0, limit)
	for rows.Next() {
		var item DeveloperAPIKey
		if err := rows.Scan(&item.ID, &item.Name, &item.Key, &item.CreatedAt); err != nil {
			continue
		}
		item.Key = maskAPIKey(item.Key)
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) CreateDeveloperAPIKey(name string) (DeveloperAPIKey, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return DeveloperAPIKey{}, &APIError{Code: "PAY-010", Message: "invalid name"}
	}
	plainKey := fmt.Sprintf("ak_live_%d", time.Now().UnixNano())
	item := DeveloperAPIKey{
		ID:        fmt.Sprintf("key_%d", time.Now().UnixNano()),
		Name:      trimmed,
		Key:       plainKey,
		CreatedAt: time.Now().UTC(),
	}
	hashed := hashAPIKey(plainKey)
	if _, err := s.store.DB.Exec(`
INSERT INTO developer_api_key (id, name, api_key, created_at)
VALUES (?, ?, ?, ?)`, item.ID, item.Name, hashed, item.CreatedAt); err != nil {
		return DeveloperAPIKey{}, err
	}
	return item, nil
}

func (s *PersistentService) ListDeveloperWebhooks(limit int) []DeveloperWebhook {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.store.DB.Query(`
SELECT id, url, event, created_at
FROM developer_webhook
ORDER BY created_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]DeveloperWebhook, 0, limit)
	for rows.Next() {
		var item DeveloperWebhook
		if err := rows.Scan(&item.ID, &item.URL, &item.Event, &item.CreatedAt); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *PersistentService) CreateDeveloperWebhook(url string, event string) (DeveloperWebhook, error) {
	trimmedURL := strings.TrimSpace(url)
	if err := validateWebhookURL(trimmedURL); err != nil {
		return DeveloperWebhook{}, err
	}
	trimmedEvent := strings.TrimSpace(event)
	if trimmedEvent == "" {
		trimmedEvent = "payment.settled"
	}
	item := DeveloperWebhook{
		ID:        fmt.Sprintf("wh_%d", time.Now().UnixNano()),
		URL:       trimmedURL,
		Event:     trimmedEvent,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := s.store.DB.Exec(`
INSERT INTO developer_webhook (id, url, event, created_at)
VALUES (?, ?, ?, ?)`, item.ID, item.URL, item.Event, item.CreatedAt); err != nil {
		return DeveloperWebhook{}, err
	}
	return item, nil
}

func (s *PersistentService) ListAPIKeys() []DeveloperAPIKey {
	return s.ListDeveloperAPIKeys(50)
}

func (s *PersistentService) CreateAPIKey(name string) (DeveloperAPIKey, error) {
	return s.CreateDeveloperAPIKey(name)
}

func (s *PersistentService) DeleteAPIKey(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return &APIError{Code: "PAY-010", Message: "invalid id"}
	}
	if _, err := s.store.DB.Exec(`DELETE FROM developer_api_key WHERE id = ?`, trimmed); err != nil {
		return err
	}
	return nil
}

func (s *PersistentService) ListWebhooks() []DeveloperWebhook {
	return s.ListDeveloperWebhooks(50)
}

func (s *PersistentService) CreateWebhook(url string, event string) (DeveloperWebhook, error) {
	return s.CreateDeveloperWebhook(url, event)
}

func (s *PersistentService) DeleteWebhook(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return &APIError{Code: "PAY-010", Message: "invalid id"}
	}
	if _, err := s.store.DB.Exec(`DELETE FROM developer_webhook WHERE id = ?`, trimmed); err != nil {
		return err
	}
	return nil
}

func (s *PersistentService) ListWebhookDeliveries(status string, event string, webhookID string, limit int, offset int) ([]WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	args := make([]any, 0, 3)
	query := `
SELECT id, webhook_id, url, event, dedupe_key, payload, status, attempts, max_attempts, next_retry_at, COALESCE(last_error, ''), created_at, updated_at
FROM webhook_delivery_task
WHERE 1 = 1`
	if trimmed := strings.ToUpper(strings.TrimSpace(status)); trimmed != "" {
		query += " AND status = ?"
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(event); trimmed != "" {
		query += " AND event = ?"
		args = append(args, trimmed)
	}
	if trimmed := strings.TrimSpace(webhookID); trimmed != "" {
		query += " AND webhook_id = ?"
		args = append(args, trimmed)
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit)
	args = append(args, offset)
	rows, err := s.store.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]WebhookDelivery, 0, limit)
	for rows.Next() {
		var item WebhookDelivery
		var payloadRaw []byte
		if err := rows.Scan(
			&item.ID, &item.WebhookID, &item.URL, &item.Event, &item.DedupeKey, &payloadRaw,
			&item.Status, &item.Attempts, &item.MaxAttempts, &item.NextRetryAt, &item.LastError, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			continue
		}
		item.Payload = json.RawMessage(payloadRaw)
		out = append(out, item)
	}
	return out, nil
}

func (s *PersistentService) ReplayWebhookDelivery(id int64) error {
	if id <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid id"}
	}
	result, err := s.store.DB.Exec(`
UPDATE webhook_delivery_task
SET status = 'PENDING',
    attempts = 0,
    next_retry_at = UTC_TIMESTAMP(),
    last_error = NULL,
    updated_at = UTC_TIMESTAMP()
WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-010", Message: "delivery not found"}
	}
	return nil
}

func (s *PersistentService) WebhookDeliveryStats() (WebhookDeliveryStats, error) {
	stats := WebhookDeliveryStats{}
	rows, err := s.store.DB.Query(`
SELECT status, COUNT(*)
FROM webhook_delivery_task
GROUP BY status`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(status)) {
		case "PENDING":
			stats.Pending += count
		case "RETRYING":
			stats.Retrying += count
		case "SENT":
			stats.Sent += count
		case "DEAD":
			stats.Dead += count
		}
		stats.Total += count
	}
	return stats, nil
}

func merchantInWhitelist(raw string, merchant string) bool {
	if raw == "" {
		return false
	}
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == ',' {
			if raw[start:i] == merchant {
				return true
			}
			start = i + 1
		}
	}
	return false
}

func formatAmount(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func (s *PersistentService) TransferFunds(fromAccountID string, toAccountID string, amount string, idemKey string) error {
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	amountV, err := parseAmount(amount)
	if err != nil || amountV <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	fromID := strings.TrimSpace(fromAccountID)
	toID := strings.TrimSpace(toAccountID)
	if fromID == "" || toID == "" || fromID == toID {
		return &APIError{Code: "PAY-010", Message: "invalid account id"}
	}
	ctx := context.Background()
	idem := "idem:fund_transfer:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return err
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
UPDATE asset_va_account
SET balance = balance - ?
WHERE va_account_id = ? AND balance >= ?`, amountV, fromID, amountV)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	res, err = tx.Exec(`
UPDATE asset_va_account
SET balance = balance + ?
WHERE va_account_id = ?`, amountV, toID)
	if err != nil {
		return err
	}
	affected, _ = res.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	tid := fmt.Sprintf("ft_%d", time.Now().UnixNano())
	if _, err := tx.Exec(`
INSERT INTO fund_transfer_order (transfer_id, from_va_account_id, to_va_account_id, amount, status, idem_key, created_at)
VALUES (?, ?, ?, ?, 'SETTLED', ?, UTC_TIMESTAMP())`, tid, fromID, toID, amountV, idemKey); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.store.Redis.Set(ctx, idem, tid, 24*time.Hour).Err()
}

func (s *PersistentService) WithdrawFunds(vaAccountID string, amount string, rail string, destinationHint string, idemKey string) (WithdrawRecord, error) {
	if strings.TrimSpace(idemKey) == "" {
		return WithdrawRecord{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	amountV, err := parseAmount(amount)
	if err != nil || amountV <= 0 {
		return WithdrawRecord{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	rail = strings.TrimSpace(rail)
	if rail == "" {
		return WithdrawRecord{}, &APIError{Code: "PAY-010", Message: "rail required"}
	}
	vaID := strings.TrimSpace(vaAccountID)
	ctx := context.Background()
	idem := "idem:fund_withdraw:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		var rec WithdrawRecord
		_ = s.store.DB.QueryRow(`
SELECT withdraw_id, va_account_id, amount, rail, destination_hint, status, idem_key, created_at
FROM fund_withdraw_order WHERE withdraw_id = ?`, found).Scan(
			&rec.WithdrawID, &rec.VAAccountID, &rec.Amount, &rec.Rail, &rec.DestinationHint, &rec.Status, &rec.IdempotencyKey, &rec.CreatedAt,
		)
		if rec.WithdrawID != "" {
			return rec, nil
		}
	} else if err != nil && err != redis.Nil {
		return WithdrawRecord{}, err
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return WithdrawRecord{}, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
UPDATE asset_va_account
SET balance = balance - ?
WHERE va_account_id = ? AND balance >= ?`, amountV, vaID, amountV)
	if err != nil {
		return WithdrawRecord{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return WithdrawRecord{}, &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	wid := fmt.Sprintf("fw_%d", time.Now().UnixNano())
	if _, err := tx.Exec(`
INSERT INTO fund_withdraw_order (withdraw_id, va_account_id, amount, rail, destination_hint, status, idem_key, created_at)
VALUES (?, ?, ?, ?, ?, 'PROCESSING', ?, UTC_TIMESTAMP())`, wid, vaID, amountV, rail, strings.TrimSpace(destinationHint), idemKey); err != nil {
		return WithdrawRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return WithdrawRecord{}, err
	}
	if err := s.store.Redis.Set(ctx, idem, wid, 24*time.Hour).Err(); err != nil {
		return WithdrawRecord{}, err
	}
	rec := WithdrawRecord{
		WithdrawID:      wid,
		VAAccountID:     vaID,
		Amount:          amountV,
		Rail:            rail,
		DestinationHint: strings.TrimSpace(destinationHint),
		Status:          "PROCESSING",
		IdempotencyKey:  idemKey,
		CreatedAt:       time.Now().UTC(),
	}
	return rec, nil
}

func (s *PersistentService) DebitPreview(agentDID string, merchantID string, amount string) (DebitPreview, error) {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	agentDID = strings.TrimSpace(agentDID)
	merchantID = strings.TrimSpace(merchantID)
	if agentDID == "" || merchantID == "" {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	var single, daily float64
	var whiteList string
	var status string
	err = s.store.DB.QueryRow(`
SELECT single_limit, daily_limit, whitelist, status
FROM pay_authorize_rule WHERE agent_did = ?`, agentDID).Scan(&single, &daily, &whiteList, &status)
	if err == sql.ErrNoRows {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	if err != nil {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "query rule failed"}
	}
	if strings.ToUpper(strings.TrimSpace(status)) != "ACTIVE" {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "authorization rule inactive"}
	}
	if v > single {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "single limit exceeded"}
	}
	var todaySpent float64
	if err := s.store.DB.QueryRow(`
SELECT COALESCE(SUM(amount), 0)
FROM pay_order
WHERE agent_did = ?
  AND status = 'SETTLED'
  AND created_at >= UTC_DATE()
  AND created_at < UTC_DATE() + INTERVAL 1 DAY`, agentDID).Scan(&todaySpent); err != nil {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "query daily spent failed"}
	}
	if todaySpent+v > daily {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "daily limit exceeded"}
	}
	if !merchantInWhitelist(whiteList, merchantID) {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "merchant not whitelisted"}
	}
	if riskErr := s.evaluateP2Risk(merchantID, v); riskErr != nil {
		return DebitPreview{}, riskErr
	}
	const feeRate = 0.003
	fee := v * feeRate
	now := time.Now().UTC()
	expires := now.Add(15 * time.Minute)
	previewID := fmt.Sprintf("prv_%d", now.UnixNano())
	_, err = s.store.DB.Exec(`
INSERT INTO payment_debit_preview (preview_id, agent_did, merchant_id, amount, fee_rate, fee_amount, net_to_merchant, fx_rate, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, UTC_TIMESTAMP())`,
		previewID, agentDID, merchantID, v, feeRate, fee, v-fee, expires)
	if err != nil {
		return DebitPreview{}, err
	}
	return DebitPreview{
		PreviewID:     previewID,
		AgentDID:      agentDID,
		MerchantID:    merchantID,
		Amount:        v,
		FeeRate:       feeRate,
		FeeAmount:     fee,
		NetToMerchant: v - fee,
		FxRate:        1,
		ExpiresAt:     expires,
		CreatedAt:     now,
	}, nil
}

func (s *PersistentService) TransferX402Outbound(vaAccountID string, toAddress string, amount string, referenceTransactionID string, idemKey string) error {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	ref := strings.TrimSpace(referenceTransactionID)
	toAddress = strings.TrimSpace(toAddress)
	if ref == "" || toAddress == "" {
		return &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	ctx := context.Background()
	idem := "idem:x402_out:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return err
	}
	var orderAmount float64
	var orderStatus string
	var agentDID string
	if err := s.store.DB.QueryRow(`
SELECT amount, status, agent_did FROM pay_order WHERE order_id = ?`, ref).Scan(&orderAmount, &orderStatus, &agentDID); err == sql.ErrNoRows {
		return &APIError{Code: "PAY-010", Message: "reference transaction not found"}
	} else if err != nil {
		return err
	}
	if orderStatus != "SETTLED" {
		return &APIError{Code: "PAY-010", Message: "reference transaction not settled"}
	}
	if math.Abs(orderAmount-v) > 1e-9 {
		return &APIError{Code: "PAY-010", Message: "amount must match reference payment"}
	}
	vaID := strings.TrimSpace(vaAccountID)
	var accAgent string
	if err := s.store.DB.QueryRow(`SELECT agent_did FROM asset_va_account WHERE va_account_id = ?`, vaID).Scan(&accAgent); err == sql.ErrNoRows {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	} else if err != nil {
		return err
	}
	if accAgent != agentDID {
		return &APIError{Code: "PAY-010", Message: "account does not match payer"}
	}
	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
UPDATE asset_va_account
SET balance = balance - ?
WHERE va_account_id = ? AND agent_did = ? AND balance >= ?`, v, vaID, agentDID, v)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	xid := fmt.Sprintf("xo_%d", time.Now().UnixNano())
	if _, err := tx.Exec(`
INSERT INTO x402_outbound_transfer (transfer_id, va_account_id, to_address, amount, reference_transaction_id, status, idem_key, created_at)
VALUES (?, ?, ?, ?, ?, 'SETTLED', ?, UTC_TIMESTAMP())`, xid, vaID, toAddress, v, ref, idemKey); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.store.Redis.Set(ctx, idem, xid, 24*time.Hour).Err()
}

func (s *PersistentService) CheckX402Settlement(transactionID string) (map[string]any, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	var status string
	var amount float64
	var merchantID string
	var createdAt time.Time
	err := s.store.DB.QueryRow(`
SELECT status, amount, merchant_id, created_at FROM pay_order WHERE order_id = ?`, transactionID).Scan(&status, &amount, &merchantID, &createdAt)
	if err == sql.ErrNoRows {
		return nil, &APIError{Code: "PAY-010", Message: "not found"}
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"transactionId": transactionID,
		"status":        status,
		"amount":        amount,
		"merchantId":    merchantID,
	}
	switch status {
	case "SETTLED":
		out["settled"] = true
		out["settledAt"] = createdAt.UTC().Format(time.RFC3339)
		out["chainTxHash"] = "sandbox:0x" + fmt.Sprintf("%x", transactionID)
	case "SETTLING":
		out["settled"] = false
	default:
		out["settled"] = false
	}
	return out, nil
}

func (s *PersistentService) RefundApply(transactionID string, reason string, idemKey string) error {
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	ctx := context.Background()
	idem := "idem:refund_apply:" + idemKey
	if found, err := s.store.Redis.Get(ctx, idem).Result(); err == nil && found != "" {
		return nil
	} else if err != nil && err != redis.Nil {
		return err
	}
	innerKey := fmt.Sprintf("apply_%s_%s", strings.TrimSpace(transactionID), idemKey)
	if err := s.Refund(transactionID, innerKey); err != nil {
		return err
	}
	return s.store.Redis.Set(ctx, idem, transactionID, 24*time.Hour).Err()
}
