package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"ai-pay-backend/internal/storage"
	"github.com/redis/go-redis/v9"
)

type PersistentService struct {
	store *storage.Store
}

func NewPersistent(store *storage.Store) *PersistentService {
	return &PersistentService{store: store}
}

func (s *PersistentService) RegisterAgent(did string) Agent {
	_, _ = s.store.DB.Exec(`INSERT IGNORE INTO agent_did (did, status, created_at) VALUES (?, 'ACTIVE', UTC_TIMESTAMP())`, did)
	return Agent{DID: did}
}

func (s *PersistentService) CreateAccount(agentDID string) Account {
	wallet := fmt.Sprintf("0xwallet_%d", time.Now().UnixNano())
	va := fmt.Sprintf("va_%d", time.Now().UnixNano())
	_, _ = s.store.DB.Exec(`
INSERT INTO asset_va_account (va_account_id, agent_did, wallet_address, balance, status, created_at)
VALUES (?, ?, ?, 0, 'ACTIVE', UTC_TIMESTAMP())`, va, agentDID, wallet)
	return Account{WalletAddress: wallet, VAAccountID: va, AgentDID: agentDID, Balance: 0}
}

func (s *PersistentService) Recharge(va string, amount string) error {
	amountV, err := parseAmount(amount)
	if err != nil || amountV <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	rechargeID := fmt.Sprintf("rch_%d", time.Now().UnixNano())
	tx, err := s.store.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "begin tx failed"}
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE asset_va_account SET balance = balance + ? WHERE va_account_id = ?`, amountV, va)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "recharge failed"}
	}
	_, err = tx.Exec(`INSERT INTO fund_recharge_order (recharge_id, va_account_id, amount, status, created_at) VALUES (?, ?, ?, 'SETTLED', UTC_TIMESTAMP())`,
		rechargeID, va, amountV)
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "recharge log failed"}
	}
	if err := tx.Commit(); err != nil {
		return &APIError{Code: "PAY-010", Message: "recharge commit failed"}
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

	tx, err := s.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "begin tx failed"}
	}
	defer tx.Rollback()

	var vaID string
	var balance float64
	err = tx.QueryRow(`SELECT va_account_id, balance FROM asset_va_account WHERE agent_did = ? FOR UPDATE`, req.PayerDID).Scan(&vaID, &balance)
	if err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if balance < amount {
		return PayResponse{}, &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	if _, err := tx.Exec(`UPDATE asset_va_account SET balance = balance - ? WHERE va_account_id = ?`, amount, vaID); err != nil {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "debit failed"}
	}
	fee := calcFee(amount)
	netAmount := amount - fee
	txnID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	if _, err := tx.Exec(`INSERT INTO pay_order (order_id, agent_did, merchant_id, amount, fee, net_amount, status, created_at) VALUES (?, ?, ?, ?, ?, ?, 'SETTLED', UTC_TIMESTAMP())`,
		txnID, req.PayerDID, req.MerchantID, amount, fee, netAmount); err != nil {
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
	if err := s.store.DB.QueryRow(`SELECT COALESCE(SUM(balance),0) FROM asset_va_account`).Scan(&totalBalance); err != nil {
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
SELECT agent_did, va_account_id, wallet_address, balance, status
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
		if err := rows.Scan(&item.AgentDID, &item.VAAccountID, &item.WalletAddress, &item.Balance, &item.Status); err != nil {
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
