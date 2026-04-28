package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

type Agent struct {
	DID       string `json:"did"`
	DIDPubKey string `json:"didPubKey,omitempty"`
}

type Account struct {
	WalletAddress string
	VAAccountID   string
	VACardNo      string
	AgentDID      string
	Balance       float64
	FrozenBalance float64
}

type AuthorizeRule struct {
	SingleLimit float64
	DailyLimit  float64
	Whitelist   map[string]struct{}
	Status      string
}

type Transaction struct {
	ID        string
	PayerDID  string
	Merchant  string
	Amount    float64
	Fee       float64
	NetAmount float64
	HoldID    string
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
	VACardNo      string  `json:"vaCardNo"`
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

type DeveloperAPIKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"createdAt"`
}

type DeveloperWebhook struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"createdAt"`
}

type WebhookDelivery struct {
	ID          int64           `json:"id"`
	WebhookID   string          `json:"webhookId"`
	URL         string          `json:"url"`
	Event       string          `json:"event"`
	DedupeKey   string          `json:"dedupeKey"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"maxAttempts"`
	NextRetryAt time.Time       `json:"nextRetryAt"`
	LastError   string          `json:"lastError,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type WebhookDeliveryStats struct {
	Pending  int `json:"pending"`
	Retrying int `json:"retrying"`
	Sent     int `json:"sent"`
	Dead     int `json:"dead"`
	Total    int `json:"total"`
}

type InterestQuote struct {
	AccountID       string    `json:"accountId"`
	AnnualRate      float64   `json:"annualRate"`
	AccruedInterest float64   `json:"accruedInterest"`
	AsOf            time.Time `json:"asOf"`
}

type VATopupConfig struct {
	AccountID         string    `json:"accountId"`
	AutoTopupEnabled  bool      `json:"autoTopupEnabled"`
	ThresholdAmount   float64   `json:"thresholdAmount"`
	TargetAmount      float64   `json:"targetAmount"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type VATransferRecord struct {
	TransferID     string    `json:"transferId"`
	FromAccountID  string    `json:"fromAccountId"`
	ToAccountID    string    `json:"toAccountId"`
	Amount         float64   `json:"amount"`
	Status         string    `json:"status"`
	IdempotencyKey string    `json:"idempotencyKey"`
	CreatedAt      time.Time `json:"createdAt"`
}

type AuditLog struct {
	ID        int64           `json:"id"`
	Actor     string          `json:"actor"`
	Role      string          `json:"role"`
	Action    string          `json:"action"`
	Resource  string          `json:"resource"`
	RequestID string          `json:"requestId"`
	Detail    json.RawMessage `json:"detail"`
	CreatedAt time.Time       `json:"createdAt"`
}

type RiskConfig struct {
	Enabled           bool      `json:"enabled"`
	SingleAmountLimit float64   `json:"singleAmountLimit"`
	BlockedMerchants  []string  `json:"blockedMerchants"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type ChannelRoute struct {
	MerchantID string    `json:"merchantId"`
	Mode       string    `json:"mode"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// FundTransferRecord documents account-to-account transfers exposed at POST /fund/transfer (M6).
type FundTransferRecord struct {
	TransferID     string    `json:"transferId"`
	FromAccountID  string    `json:"fromAccountId"`
	ToAccountID    string    `json:"toAccountId"`
	Amount         float64   `json:"amount"`
	Status         string    `json:"status"`
	IdempotencyKey string    `json:"idempotencyKey"`
	CreatedAt      time.Time `json:"createdAt"`
}

// WithdrawRecord is a fiat-rail withdrawal request placeholder (balance debited immediately in MVP).
type WithdrawRecord struct {
	WithdrawID      string    `json:"withdrawId"`
	VAAccountID     string    `json:"vaAccountId"`
	Amount          float64   `json:"amount"`
	Rail            string    `json:"rail"`
	DestinationHint string    `json:"destinationHint"`
	Status          string    `json:"status"`
	IdempotencyKey  string    `json:"idempotencyKey"`
	CreatedAt       time.Time `json:"createdAt"`
}

// DebitPreview is a fee/FX snapshot for POST /payment/debit/preview (M6).
type DebitPreview struct {
	PreviewID     string    `json:"previewId"`
	AgentDID      string    `json:"agentDid"`
	MerchantID    string    `json:"merchantId"`
	Amount        float64   `json:"amount"`
	FeeRate       float64   `json:"feeRate"`
	FeeAmount     float64   `json:"feeAmount"`
	NetToMerchant float64   `json:"netToMerchant"`
	FxRate        float64   `json:"fxRate"`
	ExpiresAt     time.Time `json:"expiresAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

// X402OutboundTransfer is an on-chain style transfer triggered after a reference x402 payment (M6).
type X402OutboundTransfer struct {
	TransferID             string    `json:"transferId"`
	VAAccountID            string    `json:"vaAccountId"`
	ToAddress              string    `json:"toAddress"`
	Amount                 float64   `json:"amount"`
	ReferenceTransactionID string    `json:"referenceTransactionId,omitempty"`
	Status                 string    `json:"status"`
	IdempotencyKey         string    `json:"idempotencyKey"`
	CreatedAt              time.Time `json:"createdAt"`
}

// VirtualCardRecord is a sandbox-only virtual card issued to an Agent DID (human/legal party).
type VirtualCardRecord struct {
	CardID           string    `json:"cardId"`
	AgentDID         string    `json:"agentDid"`
	VAAccountID      string    `json:"vaAccountId"`
	MaskedPAN        string    `json:"maskedPan"`
	Status           string    `json:"status"`
	CreditLimit      float64   `json:"creditLimit"`
	SandboxReference string    `json:"sandboxReference"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// PartyKYCStatus is verification status for the funding party identified by Agent DID (not an AI runtime).
type PartyKYCStatus struct {
	AgentDID          string    `json:"agentDid"`
	Status            string    `json:"status"`
	Tier              string    `json:"tier"`
	ExternalReference string    `json:"externalReference,omitempty"`
	VerifiedAt        time.Time `json:"verifiedAt,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// RiskAuditEntry supports compliance-oriented audit queries separate from developer audit_log.
type RiskAuditEntry struct {
	ID            int64           `json:"id"`
	Category      string          `json:"category"`
	AgentDID      string          `json:"agentDid,omitempty"`
	MerchantID    string          `json:"merchantId,omitempty"`
	TransactionID string          `json:"transactionId,omitempty"`
	Detail        json.RawMessage `json:"detail,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type Service struct {
	mu             sync.Mutex
	agents         map[string]Agent
	accounts       map[string]*Account
	accountsByVA   map[string]*Account
	accountsByCard map[string]*Account
	rules          map[string]AuthorizeRule
	orders         map[string]Transaction
	recharges      []RechargeOrder
	rechargeIdem   map[string]string
	idemMap        map[string]string
	dailySpent     map[string]float64
	holds          map[string]holdRecord
	actionIdem     map[string]struct{}
	apiKeys        []DeveloperAPIKey
	webhooks       []DeveloperWebhook
	webhookDeliver []WebhookDelivery
	webhookSeq     int64
	accountCreated map[string]time.Time
	topupConfig    map[string]VATopupConfig
	vaTransfers    []VATransferRecord
	auditLogs      []AuditLog
	auditSeq       int64
	riskConfig     RiskConfig
	channelRoutes  map[string]ChannelRoute
	fundTransfers  []FundTransferRecord
	fundWithdraws  []WithdrawRecord
	debitPreviews  map[string]DebitPreview
	x402Outbound   []X402OutboundTransfer
	virtualCards   []VirtualCardRecord
	cardPayIdem    map[string]string
	kycByAgent     map[string]PartyKYCStatus
	riskAuditEntries []RiskAuditEntry
	riskAuditSeq     int64
	walletBindings   map[string]WalletBinding
	m8sessions       map[string]m8session
	m8signReqs       map[string]m8signReq
	m8sessCreateIdem map[string]string
	m8signReqIdem    map[string]string
}

type holdRecord struct {
	ID       string
	AgentDID string
	Amount   float64
	Status   string
}

type PaymentService interface {
	RegisterAgent(did string) Agent
	SetAgentPublicKey(did string, pubKey string) error
	AgentPublicKey(did string) (string, error)
	VerifyAgentSignature(did string, message string, signature string) error
	UpdateAgentPublicKey(did string, newPubKey string, proofMessage string, proofSignature string) error
	CreateAccount(agentDID string) Account
	Recharge(va string, amount string, idemKey string) error
	SetAuthorizeRule(agentDID string, single string, daily string, merchants []string) error
	UpdateAuthorizeRule(agentDID string, single string, daily string, merchants []string) error
	FreezeAuthorizeRule(agentDID string) error
	ActivateAuthorizeRule(agentDID string) error
	Pay(req PayRequest) (PayResponse, *APIError)
	ResolveSettling(transactionID string, success bool) error
	Unfreeze(transactionID string, idemKey string) error
	Refund(transactionID string, idemKey string) error
	QueryStatus(txID string) (Transaction, error)
	BalanceByVA(va string) (float64, error)
	LedgerByVA(va string) []Transaction
	ListAgents() []AgentSummary
	ListRecharges(va string, limit int) []RechargeOrder
	OverviewMetrics() (OverviewMetrics, error)
	ListAPIKeys() []DeveloperAPIKey
	CreateAPIKey(name string) (DeveloperAPIKey, error)
	DeleteAPIKey(id string) error
	ListWebhooks() []DeveloperWebhook
	CreateWebhook(url string, event string) (DeveloperWebhook, error)
	DeleteWebhook(id string) error
	ListWebhookDeliveries(status string, event string, webhookID string, limit int, offset int) ([]WebhookDelivery, error)
	ReplayWebhookDelivery(id int64) error
	WebhookDeliveryStats() (WebhookDeliveryStats, error)
	QueryInterest(accountID string) (InterestQuote, error)
	SetVATopupConfig(accountID string, autoTopup bool, threshold string, target string) (VATopupConfig, error)
	GetVATopupConfig(accountID string) (VATopupConfig, error)
	TransferVA(fromAccountID string, toAccountID string, amount string, idemKey string) error
	ListVATransfers(accountID string, status string, startTime string, endTime string, limit int, offset int) []VATransferRecord
	AppendAuditLog(actor string, role string, action string, resource string, requestID string, detail map[string]any) error
	ListAuditLogs(action string, resource string, limit int, offset int) []AuditLog
	GetRiskConfig() RiskConfig
	SetRiskConfig(enabled bool, singleAmountLimit string, blockedMerchants []string) (RiskConfig, error)
	ListChannelRoutes() []ChannelRoute
	SetChannelRoute(merchantID string, mode string) (ChannelRoute, error)
	DeleteChannelRoute(merchantID string) error
	// M6 funds & payment extensions (gated by FEATURE_M6_FUNDS at HTTP layer).
	TransferFunds(fromAccountID string, toAccountID string, amount string, idemKey string) error
	WithdrawFunds(vaAccountID string, amount string, rail string, destinationHint string, idemKey string) (WithdrawRecord, error)
	DebitPreview(agentDID string, merchantID string, amount string) (DebitPreview, error)
	TransferX402Outbound(vaAccountID string, toAddress string, amount string, referenceTransactionID string, idemKey string) error
	CheckX402Settlement(transactionID string) (map[string]any, error)
	RefundApply(transactionID string, reason string, idemKey string) error
	// M7 card + risk + party KYC (gated by FEATURE_M7_CARD_RISK at HTTP layer).
	ApplyVirtualCard(agentDID string, vaAccountID string, requestedLimit string, idemKey string) (VirtualCardRecord, error)
	PayVirtualCard(agentDID string, cardID string, merchantID string, amount string, idemKey string) (string, error)
	ManageVirtualCard(cardID string, operation string, adjustAmount string) (VirtualCardRecord, error)
	RiskTransactionCheck(agentDID string, merchantID string, amount string, transactionID string) (map[string]any, error)
	RiskKYCVerify(agentDID string, documentReference string, idemKey string) (PartyKYCStatus, error)
	RiskAuditQuery(agentDID string, merchantID string, limit int, offset int) []RiskAuditEntry
	// M8 self-custody + session + sign workflow (FEATURE_M8_SELF_HOSTED).
	BindWallet(agentDID string, walletAddress string, label string) error
	UnbindWallet(agentDID string) error
	CreateAuthSession(agentDID string, ttlMinutes int, idemKey string) (AuthSession, error)
	RevokeAuthSession(agentDID string, sessionID string, idemKey string) error
	RequestPaymentSign(agentDID string, merchantID string, amount string, sessionID string, idemKey string) (PaymentSignRequestRecord, error)
	SubmitSignedPayment(signID string, req PayRequest) (PayResponse, *APIError)
}

func New() *Service {
	return &Service{
		agents:         map[string]Agent{},
		accounts:       map[string]*Account{},
		accountsByVA:   map[string]*Account{},
		accountsByCard: map[string]*Account{},
		rules:          map[string]AuthorizeRule{},
		orders:         map[string]Transaction{},
		recharges:      []RechargeOrder{},
		rechargeIdem:   map[string]string{},
		idemMap:        map[string]string{},
		dailySpent:     map[string]float64{},
		holds:          map[string]holdRecord{},
		actionIdem:     map[string]struct{}{},
		apiKeys:        []DeveloperAPIKey{},
		webhooks:       []DeveloperWebhook{},
		webhookDeliver: []WebhookDelivery{},
		accountCreated: map[string]time.Time{},
		topupConfig:    map[string]VATopupConfig{},
		vaTransfers:    []VATransferRecord{},
		auditLogs:      []AuditLog{},
		riskConfig: RiskConfig{
			Enabled:           true,
			SingleAmountLimit: 1000,
			BlockedMerchants:  []string{"m_risk_block"},
			UpdatedAt:         time.Now().UTC(),
		},
		channelRoutes: map[string]ChannelRoute{
			"m_fail":  {MerchantID: "m_fail", Mode: "FAIL", UpdatedAt: time.Now().UTC()},
			"m_async": {MerchantID: "m_async", Mode: "ASYNC", UpdatedAt: time.Now().UTC()},
		},
		debitPreviews:    map[string]DebitPreview{},
		cardPayIdem:      map[string]string{},
		kycByAgent:       map[string]PartyKYCStatus{},
		walletBindings:   map[string]WalletBinding{},
		m8sessions:       map[string]m8session{},
		m8signReqs:       map[string]m8signReq{},
		m8sessCreateIdem: map[string]string{},
		m8signReqIdem:    map[string]string{},
	}
}

func (s *Service) RegisterAgent(did string) Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Agent{DID: did}
	s.agents[did] = a
	return a
}

func (s *Service) SetAgentPublicKey(did string, pubKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.agents[did]
	if !ok {
		return errors.New("agent not found")
	}
	agent.DIDPubKey = pubKey
	s.agents[did] = agent
	return nil
}

func (s *Service) AgentPublicKey(did string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.agents[did]
	if !ok {
		return "", errors.New("agent not found")
	}
	return agent.DIDPubKey, nil
}

func (s *Service) VerifyAgentSignature(did string, message string, signature string) error {
	pubKey, err := s.AgentPublicKey(did)
	if err != nil {
		return errors.New("agent not found")
	}
	if strings.TrimSpace(pubKey) == "" {
		return errors.New("did public key missing")
	}
	if !verifySignature(pubKey, signature, []byte(message)) {
		return errors.New("invalid signature")
	}
	return nil
}

func (s *Service) UpdateAgentPublicKey(did string, newPubKey string, proofMessage string, proofSignature string) error {
	if strings.TrimSpace(did) == "" || strings.TrimSpace(newPubKey) == "" {
		return errors.New("invalid request")
	}
	if err := s.VerifyAgentSignature(did, proofMessage, proofSignature); err != nil {
		return err
	}
	if err := s.SetAgentPublicKey(did, newPubKey); err != nil {
		return err
	}
	return nil
}

func verifySignature(pubKeyBase64, signatureBase64 string, payload []byte) bool {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubKeyBase64))
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureBase64))
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), payload, sigBytes)
}

func (s *Service) CreateAccount(agentDID string) Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &Account{
		WalletAddress: fmt.Sprintf("0xwallet_%d", len(s.accounts)+1),
		VAAccountID:   fmt.Sprintf("va_%d", len(s.accounts)+1),
		VACardNo:      generateVACardNo(len(s.accounts) + 1),
		AgentDID:      agentDID,
		Balance:       0,
		FrozenBalance: 0,
	}
	s.accounts[agentDID] = a
	s.accountsByVA[a.VAAccountID] = a
	s.accountsByCard[a.VACardNo] = a
	s.accountCreated[a.VAAccountID] = time.Now().UTC()
	return *a
}

func (s *Service) Recharge(va string, amount string, idemKey string) error {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	if idemKey == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rechargeIdem[idemKey]; ok {
		return nil
	}
	acc, ok := s.accountsByVA[va]
	if !ok {
		acc, ok = s.accountsByCard[va]
	}
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
	s.rechargeIdem[idemKey] = va
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
		Status:      "ACTIVE",
	}
	return nil
}

func (s *Service) UpdateAuthorizeRule(agentDID string, single string, daily string, merchants []string) error {
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
	existing, ok := s.rules[agentDID]
	if !ok {
		return &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	existing.SingleLimit = singleV
	existing.DailyLimit = dailyV
	existing.Whitelist = white
	if existing.Status == "" {
		existing.Status = "ACTIVE"
	}
	s.rules[agentDID] = existing
	return nil
}

func (s *Service) FreezeAuthorizeRule(agentDID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.rules[agentDID]
	if !ok {
		return &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	existing.Status = "FROZEN"
	s.rules[agentDID] = existing
	return nil
}

func (s *Service) ActivateAuthorizeRule(agentDID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.rules[agentDID]
	if !ok {
		return &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	existing.Status = "ACTIVE"
	s.rules[agentDID] = existing
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
	if strings.ToUpper(strings.TrimSpace(rule.Status)) != "ACTIVE" {
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "authorization rule inactive"}
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
	if riskErr := s.evaluateP2Risk(req.MerchantID, amount); riskErr != nil {
		return PayResponse{}, riskErr
	}

	acc, ok := s.accounts[req.PayerDID]
	if !ok {
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	holdID, apiErr := s.freezeAmount(acc, req.PayerDID, amount)
	if apiErr != nil {
		return PayResponse{}, apiErr
	}
	switch s.decideChannelPath(req.MerchantID) {
	case channelFail:
		_ = s.releaseHold(holdID)
		return PayResponse{}, &APIError{Code: "PAY-007", Message: "channel timeout"}
	case channelSettling:
		txID := fmt.Sprintf("txn_%d", len(s.orders)+1)
		tx := Transaction{
			ID:        txID,
			PayerDID:  req.PayerDID,
			Merchant:  req.MerchantID,
			Amount:    amount,
			Fee:       0,
			NetAmount: 0,
			HoldID:    holdID,
			Status:    "SETTLING",
			CreatedAt: time.Now().UTC(),
		}
		s.orders[txID] = tx
		s.idemMap[req.IdempotencyKey] = txID
		return PayResponse{TransactionID: txID, Status: tx.Status}, nil
	}
	if err := s.debitHold(holdID); err != nil {
		_ = s.releaseHold(holdID)
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "debit failed"}
	}
	fee := calcFee(amount)
	txID := fmt.Sprintf("txn_%d", len(s.orders)+1)
	tx := Transaction{
		ID:        txID,
		PayerDID:  req.PayerDID,
		Merchant:  req.MerchantID,
		Amount:    amount,
		Fee:       fee,
		NetAmount: amount - fee,
		HoldID:    holdID,
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

func (s *Service) ResolveSettling(transactionID string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, ok := s.orders[transactionID]
	if !ok {
		return errors.New("transaction not found")
	}
	if tx.Status != "SETTLING" {
		return errors.New("transaction not settling")
	}
	if tx.HoldID == "" {
		return errors.New("missing hold id")
	}
	if success {
		if err := s.debitHold(tx.HoldID); err != nil {
			return err
		}
		tx.Fee = calcFee(tx.Amount)
		tx.NetAmount = tx.Amount - tx.Fee
		tx.Status = "SETTLED"
		s.enqueueWebhookDelivery("payment.settled", "tx:"+transactionID, map[string]any{
			"transactionId": transactionID,
			"status":        "SETTLED",
			"amount":        tx.Amount,
			"fee":           tx.Fee,
			"netAmount":     tx.NetAmount,
			"time":          time.Now().UTC().Format(time.RFC3339),
		})
	} else {
		if err := s.releaseHold(tx.HoldID); err != nil {
			return err
		}
		tx.Status = "FAILED"
	}
	s.orders[transactionID] = tx
	return nil
}

func (s *Service) Unfreeze(transactionID string, idemKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idemKey == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	if _, ok := s.actionIdem["unfreeze:"+idemKey]; ok {
		return nil
	}
	tx, ok := s.orders[transactionID]
	if !ok {
		return errors.New("transaction not found")
	}
	if tx.Status != "SETTLING" {
		return errors.New("transaction not settling")
	}
	if err := s.releaseHold(tx.HoldID); err != nil {
		return err
	}
	tx.Status = "FAILED"
	s.orders[transactionID] = tx
	s.actionIdem["unfreeze:"+idemKey] = struct{}{}
	return nil
}

func (s *Service) Refund(transactionID string, idemKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idemKey == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	if _, ok := s.actionIdem["refund:"+idemKey]; ok {
		return nil
	}
	tx, ok := s.orders[transactionID]
	if !ok {
		return errors.New("transaction not found")
	}
	if tx.Status != "SETTLED" {
		return errors.New("transaction not settled")
	}
	acc, ok := s.accounts[tx.PayerDID]
	if !ok {
		return errors.New("account not found")
	}
	acc.Balance += tx.Amount
	tx.Status = "REFUNDED"
	s.orders[transactionID] = tx
	s.enqueueWebhookDelivery("payment.refunded", "tx:"+transactionID, map[string]any{
		"transactionId": transactionID,
		"status":        "REFUNDED",
		"amount":        tx.Amount,
		"agentDid":      tx.PayerDID,
		"time":          time.Now().UTC().Format(time.RFC3339),
	})
	s.actionIdem["refund:"+idemKey] = struct{}{}
	return nil
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
			VACardNo:      acc.VACardNo,
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

func (s *Service) freezeAmount(acc *Account, agentDID string, amount float64) (string, *APIError) {
	if acc.Balance < amount {
		return "", &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	acc.Balance -= amount
	acc.FrozenBalance += amount
	holdID := fmt.Sprintf("hold_%d", len(s.holds)+1)
	s.holds[holdID] = holdRecord{
		ID:       holdID,
		AgentDID: agentDID,
		Amount:   amount,
		Status:   "FROZEN",
	}
	return holdID, nil
}

func (s *Service) debitHold(holdID string) error {
	hold, ok := s.holds[holdID]
	if !ok || hold.Status != "FROZEN" {
		return errors.New("hold not found")
	}
	acc, ok := s.accounts[hold.AgentDID]
	if !ok || acc.FrozenBalance < hold.Amount {
		return errors.New("account frozen insufficient")
	}
	acc.FrozenBalance -= hold.Amount
	hold.Status = "DEBITED"
	s.holds[holdID] = hold
	return nil
}

func (s *Service) releaseHold(holdID string) error {
	hold, ok := s.holds[holdID]
	if !ok || hold.Status != "FROZEN" {
		return errors.New("hold not found")
	}
	acc, ok := s.accounts[hold.AgentDID]
	if !ok || acc.FrozenBalance < hold.Amount {
		return errors.New("account frozen insufficient")
	}
	acc.FrozenBalance -= hold.Amount
	acc.Balance += hold.Amount
	hold.Status = "RELEASED"
	s.holds[holdID] = hold
	return nil
}

func generateVACardNo(seq int) string {
	// 16-digit virtual card number for account funding references.
	return fmt.Sprintf("68880000%08d", seq)
}

func (s *Service) ListAPIKeys() []DeveloperAPIKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := 50
	out := make([]DeveloperAPIKey, 0, limit)
	for _, item := range s.apiKeys {
		masked := item
		masked.Key = maskAPIKey(item.Key)
		out = append(out, masked)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) CreateAPIKey(name string) (DeveloperAPIKey, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return DeveloperAPIKey{}, &APIError{Code: "PAY-010", Message: "invalid api key name"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plainKey := fmt.Sprintf("ak_live_%d", time.Now().UnixNano())
	item := DeveloperAPIKey{
		ID:        fmt.Sprintf("key_%d", time.Now().UnixNano()),
		Name:      trimmed,
		Key:       plainKey,
		CreatedAt: time.Now().UTC(),
	}
	stored := item
	stored.Key = hashAPIKey(plainKey)
	s.apiKeys = append([]DeveloperAPIKey{stored}, s.apiKeys...)
	return item, nil
}

func (s *Service) DeleteAPIKey(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return &APIError{Code: "PAY-010", Message: "invalid id"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]DeveloperAPIKey, 0, len(s.apiKeys))
	found := false
	for _, item := range s.apiKeys {
		if item.ID == trimmed {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return &APIError{Code: "PAY-010", Message: "api key not found"}
	}
	s.apiKeys = next
	return nil
}

func (s *Service) ListWebhooks() []DeveloperWebhook {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := 50
	out := make([]DeveloperWebhook, 0, limit)
	for _, item := range s.webhooks {
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) CreateWebhook(url string, event string) (DeveloperWebhook, error) {
	trimmedURL := strings.TrimSpace(url)
	trimmedEvent := strings.TrimSpace(event)
	if err := validateWebhookURL(trimmedURL); err != nil {
		return DeveloperWebhook{}, err
	}
	if trimmedEvent == "" {
		trimmedEvent = "payment.settled"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item := DeveloperWebhook{
		ID:        fmt.Sprintf("wh_%d", time.Now().UnixNano()),
		URL:       trimmedURL,
		Event:     trimmedEvent,
		CreatedAt: time.Now().UTC(),
	}
	s.webhooks = append([]DeveloperWebhook{item}, s.webhooks...)
	return item, nil
}

func (s *Service) DeleteWebhook(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return &APIError{Code: "PAY-010", Message: "invalid id"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]DeveloperWebhook, 0, len(s.webhooks))
	found := false
	for _, item := range s.webhooks {
		if item.ID == trimmed {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return &APIError{Code: "PAY-010", Message: "webhook not found"}
	}
	s.webhooks = next
	return nil
}

func (s *Service) ListWebhookDeliveries(status string, event string, webhookID string, limit int, offset int) ([]WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	status = strings.ToUpper(strings.TrimSpace(status))
	event = strings.TrimSpace(event)
	webhookID = strings.TrimSpace(webhookID)
	out := make([]WebhookDelivery, 0, limit)
	skipped := 0
	for _, item := range s.webhookDeliver {
		if status != "" && strings.ToUpper(item.Status) != status {
			continue
		}
		if event != "" && item.Event != event {
			continue
		}
		if webhookID != "" && item.WebhookID != webhookID {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Service) ReplayWebhookDelivery(id int64) error {
	if id <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid id"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.webhookDeliver {
		if item.ID != id {
			continue
		}
		item.Status = "PENDING"
		item.Attempts = 0
		item.NextRetryAt = time.Now().UTC()
		item.LastError = ""
		item.UpdatedAt = time.Now().UTC()
		s.webhookDeliver[i] = item
		return nil
	}
	return &APIError{Code: "PAY-010", Message: "delivery not found"}
}

func (s *Service) WebhookDeliveryStats() (WebhookDeliveryStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := WebhookDeliveryStats{}
	for _, item := range s.webhookDeliver {
		switch strings.ToUpper(strings.TrimSpace(item.Status)) {
		case "PENDING":
			stats.Pending++
		case "RETRYING":
			stats.Retrying++
		case "SENT":
			stats.Sent++
		case "DEAD":
			stats.Dead++
		}
		stats.Total++
	}
	return stats, nil
}

func (s *Service) QueryInterest(accountID string) (InterestQuote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	accountID = strings.TrimSpace(accountID)
	acc, ok := s.accountsByVA[accountID]
	if !ok {
		return InterestQuote{}, errors.New("account not found")
	}
	createdAt := s.accountCreated[accountID]
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	now := time.Now().UTC()
	days := now.Sub(createdAt).Hours() / 24
	if days < 0 {
		days = 0
	}
	const annualRate = 0.03
	accrued := acc.Balance * annualRate * (days / 365.0)
	return InterestQuote{
		AccountID:       accountID,
		AnnualRate:      annualRate,
		AccruedInterest: accrued,
		AsOf:            now,
	}, nil
}

func (s *Service) SetVATopupConfig(accountID string, autoTopup bool, threshold string, target string) (VATopupConfig, error) {
	thresholdV, err := parseAmount(threshold)
	if err != nil || thresholdV < 0 {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "invalid threshold amount"}
	}
	targetV, err := parseAmount(target)
	if err != nil || targetV <= 0 || targetV < thresholdV {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "invalid target amount"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	accountID = strings.TrimSpace(accountID)
	if _, ok := s.accountsByVA[accountID]; !ok {
		return VATopupConfig{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	item := VATopupConfig{
		AccountID:        accountID,
		AutoTopupEnabled: autoTopup,
		ThresholdAmount:  thresholdV,
		TargetAmount:     targetV,
		UpdatedAt:        time.Now().UTC(),
	}
	s.topupConfig[accountID] = item
	return item, nil
}

func (s *Service) GetVATopupConfig(accountID string) (VATopupConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	accountID = strings.TrimSpace(accountID)
	if item, ok := s.topupConfig[accountID]; ok {
		return item, nil
	}
	if _, ok := s.accountsByVA[accountID]; !ok {
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

func (s *Service) TransferVA(fromAccountID string, toAccountID string, amount string, idemKey string) error {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "va_transfer:" + idemKey
	if _, ok := s.actionIdem[key]; ok {
		return nil
	}
	from := s.accountsByVA[strings.TrimSpace(fromAccountID)]
	to := s.accountsByVA[strings.TrimSpace(toAccountID)]
	if from == nil || to == nil {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if from.VAAccountID == to.VAAccountID {
		return &APIError{Code: "PAY-010", Message: "cannot transfer to same account"}
	}
	if from.Balance < v {
		return &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	from.Balance -= v
	to.Balance += v
	s.vaTransfers = append([]VATransferRecord{
		{
			TransferID:     fmt.Sprintf("vat_%d", time.Now().UnixNano()),
			FromAccountID:  from.VAAccountID,
			ToAccountID:    to.VAAccountID,
			Amount:         v,
			Status:         "SETTLED",
			IdempotencyKey: strings.TrimSpace(idemKey),
			CreatedAt:      time.Now().UTC(),
		},
	}, s.vaTransfers...)
	s.actionIdem[key] = struct{}{}
	return nil
}

func (s *Service) ListVATransfers(accountID string, status string, startTime string, endTime string, limit int, offset int) []VATransferRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	accountID = strings.TrimSpace(accountID)
	status = strings.ToUpper(strings.TrimSpace(status))
	startAt := parseOptionalRFC3339(startTime)
	endAt := parseOptionalRFC3339(endTime)
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	out := make([]VATransferRecord, 0, limit)
	skipped := 0
	for _, item := range s.vaTransfers {
		if accountID != "" && item.FromAccountID != accountID && item.ToAccountID != accountID {
			continue
		}
		if status != "" && strings.ToUpper(item.Status) != status {
			continue
		}
		if startAt != nil && item.CreatedAt.Before(*startAt) {
			continue
		}
		if endAt != nil && item.CreatedAt.After(*endAt) {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) AppendAuditLog(actor string, role string, action string, resource string, requestID string, detail map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	s.auditSeq++
	s.auditLogs = append([]AuditLog{
		{
			ID:        s.auditSeq,
			Actor:     strings.TrimSpace(actor),
			Role:      strings.TrimSpace(role),
			Action:    strings.TrimSpace(action),
			Resource:  strings.TrimSpace(resource),
			RequestID: strings.TrimSpace(requestID),
			Detail:    json.RawMessage(raw),
			CreatedAt: time.Now().UTC(),
		},
	}, s.auditLogs...)
	return nil
}

func (s *Service) ListAuditLogs(action string, resource string, limit int, offset int) []AuditLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	action = strings.TrimSpace(action)
	resource = strings.TrimSpace(resource)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	out := make([]AuditLog, 0, limit)
	skipped := 0
	for _, item := range s.auditLogs {
		if action != "" && item.Action != action {
			continue
		}
		if resource != "" && item.Resource != resource {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) GetRiskConfig() RiskConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.riskConfig
	out.BlockedMerchants = append([]string{}, s.riskConfig.BlockedMerchants...)
	return out
}

func (s *Service) SetRiskConfig(enabled bool, singleAmountLimit string, blockedMerchants []string) (RiskConfig, error) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.riskConfig = RiskConfig{
		Enabled:           enabled,
		SingleAmountLimit: limit,
		BlockedMerchants:  trimmed,
		UpdatedAt:         time.Now().UTC(),
	}
	return s.riskConfig, nil
}

func (s *Service) ListChannelRoutes() []ChannelRoute {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ChannelRoute, 0, len(s.channelRoutes))
	for _, item := range s.channelRoutes {
		out = append(out, item)
	}
	return out
}

func (s *Service) SetChannelRoute(merchantID string, mode string) (ChannelRoute, error) {
	merchantID = strings.TrimSpace(merchantID)
	mode = strings.ToUpper(strings.TrimSpace(mode))
	if merchantID == "" || !isValidChannelMode(mode) {
		return ChannelRoute{}, &APIError{Code: "PAY-010", Message: "invalid channel route"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item := ChannelRoute{MerchantID: merchantID, Mode: mode, UpdatedAt: time.Now().UTC()}
	s.channelRoutes[merchantID] = item
	return item, nil
}

func (s *Service) DeleteChannelRoute(merchantID string) error {
	merchantID = strings.TrimSpace(merchantID)
	if merchantID == "" {
		return &APIError{Code: "PAY-010", Message: "invalid merchantId"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.channelRoutes, merchantID)
	return nil
}

func (s *Service) TransferFunds(fromAccountID string, toAccountID string, amount string, idemKey string) error {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "fund_transfer:" + idemKey
	if _, ok := s.actionIdem[key]; ok {
		return nil
	}
	from := s.accountsByVA[strings.TrimSpace(fromAccountID)]
	to := s.accountsByVA[strings.TrimSpace(toAccountID)]
	if from == nil || to == nil {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if from.VAAccountID == to.VAAccountID {
		return &APIError{Code: "PAY-010", Message: "cannot transfer to same account"}
	}
	if from.Balance < v {
		return &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	from.Balance -= v
	to.Balance += v
	rec := FundTransferRecord{
		TransferID:     fmt.Sprintf("ft_%d", time.Now().UnixNano()),
		FromAccountID:  from.VAAccountID,
		ToAccountID:    to.VAAccountID,
		Amount:         v,
		Status:         "SETTLED",
		IdempotencyKey: strings.TrimSpace(idemKey),
		CreatedAt:      time.Now().UTC(),
	}
	s.fundTransfers = append([]FundTransferRecord{rec}, s.fundTransfers...)
	s.actionIdem[key] = struct{}{}
	return nil
}

func (s *Service) WithdrawFunds(vaAccountID string, amount string, rail string, destinationHint string, idemKey string) (WithdrawRecord, error) {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return WithdrawRecord{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	if strings.TrimSpace(idemKey) == "" {
		return WithdrawRecord{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	rail = strings.TrimSpace(rail)
	if rail == "" {
		return WithdrawRecord{}, &APIError{Code: "PAY-010", Message: "rail required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "fund_withdraw:" + idemKey
	if _, ok := s.actionIdem[key]; ok {
		for _, w := range s.fundWithdraws {
			if w.IdempotencyKey == strings.TrimSpace(idemKey) {
				return w, nil
			}
		}
		return WithdrawRecord{}, &APIError{Code: "PAY-010", Message: "withdraw idempotency replay without record"}
	}
	acc := s.accountsByVA[strings.TrimSpace(vaAccountID)]
	if acc == nil {
		return WithdrawRecord{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if acc.Balance < v {
		return WithdrawRecord{}, &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	acc.Balance -= v
	rec := WithdrawRecord{
		WithdrawID:      fmt.Sprintf("fw_%d", time.Now().UnixNano()),
		VAAccountID:     acc.VAAccountID,
		Amount:          v,
		Rail:            rail,
		DestinationHint: strings.TrimSpace(destinationHint),
		Status:          "PROCESSING",
		IdempotencyKey:  strings.TrimSpace(idemKey),
		CreatedAt:       time.Now().UTC(),
	}
	s.fundWithdraws = append([]WithdrawRecord{rec}, s.fundWithdraws...)
	s.actionIdem[key] = struct{}{}
	return rec, nil
}

func (s *Service) DebitPreview(agentDID string, merchantID string, amount string) (DebitPreview, error) {
	v, err := parseAmount(amount)
	if err != nil || v <= 0 {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	agentDID = strings.TrimSpace(agentDID)
	merchantID = strings.TrimSpace(merchantID)
	if agentDID == "" || merchantID == "" {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[agentDID]; !ok {
		return DebitPreview{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	rule, ok := s.rules[agentDID]
	if !ok {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "missing authorization rule"}
	}
	if strings.ToUpper(strings.TrimSpace(rule.Status)) != "ACTIVE" {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "authorization rule inactive"}
	}
	if v > rule.SingleLimit {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "single limit exceeded"}
	}
	todayKey := agentDID + ":" + time.Now().UTC().Format("2006-01-02")
	if s.dailySpent[todayKey]+v > rule.DailyLimit {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "daily limit exceeded"}
	}
	if _, ok := rule.Whitelist[merchantID]; !ok {
		return DebitPreview{}, &APIError{Code: "PAY-002", Message: "merchant not whitelisted"}
	}
	if riskErr := evaluateRiskWithConfig(s.riskConfig, merchantID, v); riskErr != nil {
		return DebitPreview{}, riskErr
	}
	const feeRate = 0.003
	fee := v * feeRate
	now := time.Now().UTC()
	pv := DebitPreview{
		PreviewID:     fmt.Sprintf("prv_%d", now.UnixNano()),
		AgentDID:      agentDID,
		MerchantID:    merchantID,
		Amount:        v,
		FeeRate:       feeRate,
		FeeAmount:     fee,
		NetToMerchant: v - fee,
		FxRate:        1,
		ExpiresAt:     now.Add(15 * time.Minute),
		CreatedAt:     now,
	}
	s.debitPreviews[pv.PreviewID] = pv
	return pv, nil
}

func (s *Service) TransferX402Outbound(vaAccountID string, toAddress string, amount string, referenceTransactionID string, idemKey string) error {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "x402_out:" + idemKey
	if _, ok := s.actionIdem[key]; ok {
		return nil
	}
	tx, ok := s.orders[ref]
	if !ok {
		return &APIError{Code: "PAY-010", Message: "reference transaction not found"}
	}
	if tx.Status != "SETTLED" {
		return &APIError{Code: "PAY-010", Message: "reference transaction not settled"}
	}
	if math.Abs(tx.Amount-v) > 1e-9 {
		return &APIError{Code: "PAY-010", Message: "amount must match reference payment"}
	}
	acc := s.accountsByVA[strings.TrimSpace(vaAccountID)]
	if acc == nil {
		return &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if acc.AgentDID != tx.PayerDID {
		return &APIError{Code: "PAY-010", Message: "account does not match payer"}
	}
	if acc.Balance < v {
		return &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	acc.Balance -= v
	out := X402OutboundTransfer{
		TransferID:             fmt.Sprintf("xo_%d", time.Now().UnixNano()),
		VAAccountID:            acc.VAAccountID,
		ToAddress:              toAddress,
		Amount:                 v,
		ReferenceTransactionID: ref,
		Status:                 "SETTLED",
		IdempotencyKey:         strings.TrimSpace(idemKey),
		CreatedAt:              time.Now().UTC(),
	}
	s.x402Outbound = append([]X402OutboundTransfer{out}, s.x402Outbound...)
	s.actionIdem[key] = struct{}{}
	return nil
}

func (s *Service) CheckX402Settlement(transactionID string) (map[string]any, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, ok := s.orders[transactionID]
	if !ok {
		return nil, &APIError{Code: "PAY-010", Message: "not found"}
	}
	out := map[string]any{
		"transactionId": transactionID,
		"status":        tx.Status,
		"amount":        tx.Amount,
		"merchantId":    tx.Merchant,
	}
	switch tx.Status {
	case "SETTLED":
		out["settled"] = true
		out["settledAt"] = tx.CreatedAt.UTC().Format(time.RFC3339)
		out["chainTxHash"] = "sandbox:0x" + fmt.Sprintf("%x", transactionID)
	case "SETTLING":
		out["settled"] = false
	default:
		out["settled"] = false
	}
	return out, nil
}

func (s *Service) RefundApply(transactionID string, reason string, idemKey string) error {
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	innerKey := "refund_apply_inner:" + idemKey
	s.mu.Lock()
	if _, ok := s.actionIdem["refund_apply:"+idemKey]; ok {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	if err := s.Refund(transactionID, innerKey); err != nil {
		return err
	}
	s.mu.Lock()
	s.actionIdem["refund_apply:"+idemKey] = struct{}{}
	s.mu.Unlock()
	return nil
}

func parseOptionalRFC3339(raw string) *time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil
	}
	v := t.UTC()
	return &v
}

func (s *Service) enqueueWebhookDelivery(event, dedupeKey string, payload map[string]any) {
	if len(s.webhooks) == 0 {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, wh := range s.webhooks {
		if wh.Event != event {
			continue
		}
		s.webhookSeq++
		item := WebhookDelivery{
			ID:          s.webhookSeq,
			WebhookID:   wh.ID,
			URL:         wh.URL,
			Event:       event,
			DedupeKey:   dedupeKey,
			Payload:     json.RawMessage(body),
			Status:      "PENDING",
			Attempts:    0,
			MaxAttempts: 5,
			NextRetryAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.webhookDeliver = append([]WebhookDelivery{item}, s.webhookDeliver...)
	}
}
