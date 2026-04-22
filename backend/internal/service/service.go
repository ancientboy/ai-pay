package service

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

type Agent struct {
	DID string
}

type Account struct {
	WalletAddress string
	VAAccountID   string
	AgentDID      string
	Balance       float64
}

type AuthorizeRule struct {
	SingleLimit float64
	DailyLimit  float64
	Whitelist   map[string]struct{}
}

type Transaction struct {
	ID        string
	PayerDID  string
	Merchant  string
	Amount    float64
	Fee       float64
	NetAmount float64
	Status    string
	CreatedAt time.Time
}

type PayRequest struct {
	PayerDID       string
	MerchantID     string
	Amount         string
	IdempotencyKey string
	Signature      string
}

type PayResponse struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
}

type AgentSummary struct {
	AgentDID      string  `json:"agentDid"`
	VAAccountID   string  `json:"vaAccountId"`
	WalletAddress string  `json:"walletAddress"`
	Balance       float64 `json:"balance"`
	Status        string  `json:"status"`
}

type RechargeOrder struct {
	RechargeID  string    `json:"rechargeId"`
	VAAccountID string    `json:"vaAccountId"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type OverviewMetrics struct {
	TotalBalance       float64 `json:"totalBalance"`
	TodaySpend         float64 `json:"todaySpend"`
	PaymentSuccessRate float64 `json:"paymentSuccessRate"`
	AlertCount         int     `json:"alertCount"`
}

type Service struct {
	mu           sync.Mutex
	agents       map[string]Agent
	accounts     map[string]*Account
	accountsByVA map[string]*Account
	rules        map[string]AuthorizeRule
	orders       map[string]Transaction
	recharges    []RechargeOrder
	idemMap      map[string]string
	dailySpent   map[string]float64
}

type PaymentService interface {
	RegisterAgent(did string) Agent
	CreateAccount(agentDID string) Account
	Recharge(va string, amount string) error
	SetAuthorizeRule(agentDID string, single string, daily string, merchants []string) error
	Pay(req PayRequest) (PayResponse, *APIError)
	QueryStatus(txID string) (Transaction, error)
	BalanceByVA(va string) (float64, error)
	LedgerByVA(va string) []Transaction
	ListAgents() []AgentSummary
	ListRecharges(va string, limit int) []RechargeOrder
	OverviewMetrics() (OverviewMetrics, error)
}

func New() *Service {
	return &Service{
		agents:       map[string]Agent{},
		accounts:     map[string]*Account{},
		accountsByVA: map[string]*Account{},
		rules:        map[string]AuthorizeRule{},
		orders:       map[string]Transaction{},
		recharges:    []RechargeOrder{},
		idemMap:      map[string]string{},
		dailySpent:   map[string]float64{},
	}
}

func (s *Service) RegisterAgent(did string) Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Agent{DID: did}
	s.agents[did] = a
	return a
}

func (s *Service) CreateAccount(agentDID string) Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &Account{
		WalletAddress: fmt.Sprintf("0xwallet_%d", len(s.accounts)+1),
		VAAccountID:   fmt.Sprintf("va_%d", len(s.accounts)+1),
		AgentDID:      agentDID,
		Balance:       0,
	}
	s.accounts[agentDID] = a
	s.accountsByVA[a.VAAccountID] = a
	return *a
}

func (s *Service) Recharge(va string, amount string) error {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accountsByVA[va]
	if !ok {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	acc.Balance += v
	s.recharges = append([]RechargeOrder{
		{
			RechargeID:  fmt.Sprintf("rch_%d", len(s.recharges)+1),
			VAAccountID: va,
			Amount:      v,
			Status:      "SETTLED",
			CreatedAt:   time.Now().UTC(),
		},
	}, s.recharges...)
	return nil
}

func (s *Service) SetAuthorizeRule(agentDID string, single string, daily string, merchants []string) error {
	singleV, err := parseAmount(single)
	if err != nil {
		return err
	}
	dailyV, err := parseAmount(daily)
	if err != nil {
		return err
	}
	white := make(map[string]struct{}, len(merchants))
	for _, m := range merchants {
		white[m] = struct{}{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[agentDID] = AuthorizeRule{
		SingleLimit: singleV,
		DailyLimit:  dailyV,
		Whitelist:   white,
	}
	return nil
}

func (s *Service) Pay(req PayRequest) (PayResponse, *APIError) {
	if req.Signature == "" {
		return PayResponse{}, &APIError{Code: "PAY-001", Message: "signature required"}
	}
	amount, err := parseAmount(req.Amount)
	if err != nil || amount <= 0 {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if txID, ok := s.idemMap[req.IdempotencyKey]; ok {
		tx := s.orders[txID]
		return PayResponse{TransactionID: tx.ID, Status: tx.Status}, nil
	}

	rule, ok := s.rules[req.PayerDID]
	if !ok {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	if amount > rule.SingleLimit {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "single limit exceeded"}
	}
	todayKey := req.PayerDID + ":" + time.Now().UTC().Format("2006-01-02")
	if s.dailySpent[todayKey]+amount > rule.DailyLimit {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "daily limit exceeded"}
	}
	if _, ok := rule.Whitelist[req.MerchantID]; !ok {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "merchant not whitelisted"}
	}

	acc, ok := s.accounts[req.PayerDID]
	if !ok {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if acc.Balance < amount {
		return PayResponse{}, &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}

	acc.Balance -= amount
	fee := calcFee(amount)
	txID := fmt.Sprintf("txn_%d", len(s.orders)+1)
	tx := Transaction{
		ID:        txID,
		PayerDID:  req.PayerDID,
		Merchant:  req.MerchantID,
		Amount:    amount,
		Fee:       fee,
		NetAmount: amount - fee,
		Status:    "SETTLED",
		CreatedAt: time.Now().UTC(),
	}
	s.orders[txID] = tx
	s.idemMap[req.IdempotencyKey] = txID
	s.dailySpent[todayKey] += amount
	return PayResponse{TransactionID: txID, Status: tx.Status}, nil
}

func (s *Service) QueryStatus(txID string) (Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, ok := s.orders[txID]
	if !ok {
		return Transaction{}, errors.New("not found")
	}
	return tx, nil
}

func (s *Service) BalanceByVA(va string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accountsByVA[va]
	if !ok {
		return 0, errors.New("not found")
	}
	return acc.Balance, nil
}

func (s *Service) LedgerByVA(va string) []Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()
	acc, ok := s.accountsByVA[va]
	if !ok {
		return nil
	}
	result := make([]Transaction, 0)
	for _, tx := range s.orders {
		if tx.PayerDID == acc.AgentDID {
			result = append(result, tx)
		}
	}
	return result
}

func (s *Service) OverviewMetrics() (OverviewMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	totalBalance := 0.0
	for _, acc := range s.accountsByVA {
		totalBalance += acc.Balance
	}

	today := time.Now().UTC().Format("2006-01-02")
	todaySpend := 0.0
	totalOrders := 0
	successOrders := 0
	for _, tx := range s.orders {
		totalOrders++
		if tx.Status == "SETTLED" {
			successOrders++
		}
		if tx.CreatedAt.UTC().Format("2006-01-02") == today && tx.Status == "SETTLED" {
			todaySpend += tx.Amount
		}
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

func (s *Service) ListAgents() []AgentSummary {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]AgentSummary, 0, len(s.accounts))
	for _, acc := range s.accounts {
		out = append(out, AgentSummary{
			AgentDID:      acc.AgentDID,
			VAAccountID:   acc.VAAccountID,
			WalletAddress: acc.WalletAddress,
			Balance:       acc.Balance,
			Status:        "ACTIVE",
		})
	}
	return out
}

func (s *Service) ListRecharges(va string, limit int) []RechargeOrder {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	out := make([]RechargeOrder, 0, limit)
	for _, r := range s.recharges {
		if va != "" && r.VAAccountID != va {
			continue
		}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func parseAmount(v string) (float64, error) {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func calcFee(amount float64) float64 {
	const feeRate = 0.003
	return amount * feeRate
}
