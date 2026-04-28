package http

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-pay-backend/internal/service"
)

type Server struct {
	svc              service.PaymentService
	nowFn            func() time.Time
	readyCheck       func(context.Context) error
	adminBearerToken string
	readonlyToken    string
	callbackToken    string
	callbackSignKey  string
	callbackNonceLRU *fixedWindowLimiter
	callbackDedupe   *callbackDedupeStore
	signatureMaxSkew time.Duration
	ipRateLimiter    rateLimiter
	agentRateLimiter rateLimiter
	agentOwnersMu    sync.RWMutex
	agentOwners      map[string]string
}

type contextKey string

const requestIDKey contextKey = "requestId"
const userIDHeader = "X-User-Id"
const userRoleHeader = "X-User-Role"

func NewServer(svc service.PaymentService) *Server {
	s := &Server{
		svc:              svc,
		nowFn:            time.Now,
		signatureMaxSkew: 5 * time.Minute,
		callbackNonceLRU: newFixedWindowLimiter(1, 10*time.Minute),
		callbackDedupe:   newCallbackDedupeStore(10 * time.Minute),
		ipRateLimiter:    newFixedWindowLimiter(120, time.Minute),
		agentRateLimiter: newFixedWindowLimiter(60, time.Minute),
		agentOwners:      map[string]string{},
	}
	return s
}

func NewServerWithReadiness(svc service.PaymentService, readyCheck func(context.Context) error) *Server {
	s := NewServer(svc)
	s.readyCheck = readyCheck
	return s
}

func (s *Server) SetAdminBearerToken(token string) {
	s.adminBearerToken = strings.TrimSpace(token)
}

func (s *Server) SetReadonlyBearerToken(token string) {
	s.readonlyToken = strings.TrimSpace(token)
}

func (s *Server) SetCallbackToken(token string) {
	s.callbackToken = strings.TrimSpace(token)
}

func (s *Server) SetCallbackSigningSecret(secret string) {
	s.callbackSignKey = strings.TrimSpace(secret)
}

func NewServerForTest(svc service.PaymentService, nowFn func() time.Time, ipLimit, agentLimit int) *Server {
	s := NewServer(svc)
	s.nowFn = nowFn
	s.ipRateLimiter = newFixedWindowLimiter(ipLimit, time.Minute)
	s.agentRateLimiter = newFixedWindowLimiter(agentLimit, time.Minute)
	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("POST /agent/did/register", s.handleRegisterAgent)
	mux.HandleFunc("POST /agent/did/verify", s.handleVerifyAgent)
	mux.HandleFunc("POST /agent/did/update", s.handleUpdateAgent)
	mux.HandleFunc("POST /account/create", s.handleCreateAccount)
	mux.HandleFunc("GET /agent/list", s.handleAgentList)
	mux.HandleFunc("POST /fund/recharge", s.handleRecharge)
	mux.Handle("GET /fund/recharge/address", s.withReadAuth(http.HandlerFunc(s.handleRechargeAddressQuery)))
	mux.Handle("POST /fund/recharge/callback", s.withCallbackToken(http.HandlerFunc(s.handleRechargeCallback)))
	mux.HandleFunc("GET /fund/recharge/list", s.handleRechargeList)
	mux.HandleFunc("POST /authorize/payment/set", s.handleAuthorizeSet)
	mux.HandleFunc("POST /authorize/payment/update", s.handleAuthorizeUpdate)
	mux.HandleFunc("POST /authorize/freeze", s.handleAuthorizeFreeze)
	mux.HandleFunc("POST /authorize/activate", s.handleAuthorizeActivate)
	mux.HandleFunc("POST /payment/x402/pay", s.handlePay)
	mux.Handle("POST /payment/status/callback", s.withCallbackToken(http.HandlerFunc(s.handleStatusCallback)))
	mux.Handle("POST /payment/unfreeze", s.withAdminAuth(http.HandlerFunc(s.handleUnfreeze)))
	mux.Handle("POST /payment/refund", s.withAdminAuth(http.HandlerFunc(s.handleRefund)))
	mux.HandleFunc("GET /payment/status/query", s.handleStatus)
	mux.HandleFunc("GET /account/balance/query", s.handleBalance)
	mux.HandleFunc("GET /account/ledger/query", s.handleLedger)
	mux.Handle("GET /account/interest/query", s.withReadAuth(http.HandlerFunc(s.handleInterest)))
	mux.Handle("POST /account/va/topup/config", s.withAdminAuth(http.HandlerFunc(s.handleVATopupConfigSet)))
	mux.Handle("GET /account/va/topup/config", s.withReadAuth(http.HandlerFunc(s.handleVATopupConfigGet)))
	mux.Handle("POST /account/va/transfer", s.withAdminAuth(http.HandlerFunc(s.handleVATransfer)))
	mux.Handle("GET /account/va/transfer/list", s.withReadAuth(http.HandlerFunc(s.handleVATransferList)))
	mux.HandleFunc("GET /metrics/overview", s.handleOverviewMetrics)
	mux.Handle("GET /developer/api-keys", s.withReadAuth(http.HandlerFunc(s.handleAPIKeyList)))
	mux.Handle("POST /developer/api-keys", s.withAdminAuth(http.HandlerFunc(s.handleAPIKeyCreate)))
	mux.Handle("DELETE /developer/api-keys", s.withAdminAuth(http.HandlerFunc(s.handleAPIKeyDelete)))
	mux.Handle("GET /developer/webhooks", s.withReadAuth(http.HandlerFunc(s.handleWebhookList)))
	mux.Handle("POST /developer/webhooks", s.withAdminAuth(http.HandlerFunc(s.handleWebhookCreate)))
	mux.Handle("DELETE /developer/webhooks", s.withAdminAuth(http.HandlerFunc(s.handleWebhookDelete)))
	mux.Handle("GET /developer/webhook-deliveries", s.withReadAuth(http.HandlerFunc(s.handleWebhookDeliveryList)))
	mux.Handle("GET /developer/webhook-deliveries/stats", s.withReadAuth(http.HandlerFunc(s.handleWebhookDeliveryStats)))
	mux.Handle("POST /developer/webhook-deliveries/replay", s.withAdminAuth(http.HandlerFunc(s.handleWebhookDeliveryReplay)))
	mux.Handle("GET /developer/audit-logs", s.withReadAuth(http.HandlerFunc(s.handleAuditLogList)))
	mux.Handle("GET /developer/risk-config", s.withReadAuth(http.HandlerFunc(s.handleRiskConfigGet)))
	mux.Handle("POST /developer/risk-config", s.withAdminAuth(http.HandlerFunc(s.handleRiskConfigSet)))
	mux.Handle("GET /developer/channel-routes", s.withReadAuth(http.HandlerFunc(s.handleChannelRouteList)))
	mux.Handle("POST /developer/channel-routes", s.withAdminAuth(http.HandlerFunc(s.handleChannelRouteSet)))
	mux.Handle("DELETE /developer/channel-routes", s.withAdminAuth(http.HandlerFunc(s.handleChannelRouteDelete)))
	mux.Handle("GET /developer/stablecoin-config", s.withReadAuth(http.HandlerFunc(s.handleStablecoinConfigList)))
	mux.Handle("POST /developer/stablecoin-config", s.withAdminAuth(http.HandlerFunc(s.handleStablecoinConfigSet)))
	mux.Handle("GET /fund/recharge/confirm", s.withReadAuth(http.HandlerFunc(s.handleRechargeConfirmQuery)))
	mux.Handle("POST /fund/transfer", s.withM6Funds(s.withAdminAuth(http.HandlerFunc(s.handleFundTransfer))))
	mux.Handle("POST /fund/withdraw", s.withM6Funds(s.withAdminAuth(http.HandlerFunc(s.handleFundWithdraw))))
	mux.Handle("POST /payment/debit/preview", s.withM6Funds(http.HandlerFunc(s.handleDebitPreview)))
	mux.Handle("POST /payment/x402/check", s.withM6Funds(http.HandlerFunc(s.handleX402Check)))
	mux.Handle("POST /payment/x402/transfer", s.withM6Funds(s.withAdminAuth(http.HandlerFunc(s.handleX402Transfer))))
	mux.Handle("POST /payment/refund/apply", s.withM6Funds(http.HandlerFunc(s.handleRefundApply)))
	mux.Handle("POST /payment/card/apply", s.withM7CardRisk(s.withAdminAuth(http.HandlerFunc(s.handleCardApply))))
	mux.Handle("POST /payment/card/pay", s.withM7CardRisk(http.HandlerFunc(s.handleCardPay)))
	mux.Handle("PUT /payment/card/manage", s.withM7CardRisk(s.withAdminAuth(http.HandlerFunc(s.handleCardManage))))
	mux.Handle("POST /risk/transaction/check", s.withM7CardRisk(s.withReadAuth(http.HandlerFunc(s.handleRiskTransactionCheck))))
	mux.Handle("POST /risk/kyc/verify", s.withM7CardRisk(http.HandlerFunc(s.handleRiskKYCVerify)))
	mux.Handle("GET /risk/audit/query", s.withM7CardRisk(s.withReadAuth(http.HandlerFunc(s.handleRiskAuditQuery))))
	mux.Handle("POST /wallet/bind", s.withM8SelfHosted(http.HandlerFunc(s.handleWalletBind)))
	mux.Handle("POST /wallet/unbind", s.withM8SelfHosted(http.HandlerFunc(s.handleWalletUnbind)))
	mux.Handle("POST /authorize/session/create", s.withM8SelfHosted(http.HandlerFunc(s.handleSessionCreate)))
	mux.Handle("POST /authorize/session/revoke", s.withM8SelfHosted(http.HandlerFunc(s.handleSessionRevoke)))
	mux.Handle("POST /payment/sign/request", s.withM8SelfHosted(http.HandlerFunc(s.handlePaymentSignRequest)))
	mux.Handle("POST /payment/sign/submit", s.withM8SelfHosted(http.HandlerFunc(s.handlePaymentSignSubmit)))
	mux.HandleFunc("GET /subscription/plans", s.handleSubscriptionPlans)
	mux.HandleFunc("GET /subscription/current", s.handleSubscriptionCurrent)
	mux.HandleFunc("GET /billing/invoices", s.handleBillingInvoices)
	mux.Handle("POST /subscription/change", s.withReadAuth(http.HandlerFunc(s.handleSubscriptionChange)))
	mux.Handle("GET /admin/subscriptions", s.withAdminAuth(http.HandlerFunc(s.handleAdminSubscriptions)))
	mux.Handle("POST /admin/subscriptions/adjust", s.withAdminAuth(http.HandlerFunc(s.handleAdminSubscriptionAdjust)))
	return s.withRequestID(mux)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, err *service.APIError) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"code":    err.Code,
		"message": err.Message,
	})
}

func writeInternalError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	log.Printf("requestId=%s op=%s err=%v", getRequestID(r.Context()), operation, err)
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"code":    "PAY-010",
		"message": operation + " failed",
	})
}

type registerAgentReq struct {
	AgentDID  string `json:"agentDid"`
	DIDPubKey string `json:"didPubKey"`
}

func (s *Server) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req registerAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || strings.TrimSpace(req.DIDPubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !isBase64Ed25519PubKey(req.DIDPubKey) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid did pub key"})
		return
	}
	agent := s.svc.RegisterAgent(req.AgentDID)
	s.bindAgentOwner(req.AgentDID, s.requestUserID(r))
	if err := s.svc.SetAgentPublicKey(req.AgentDID, req.DIDPubKey); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "save did pub key failed"})
		return
	}
	agent.DIDPubKey = req.DIDPubKey
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": agent})
}

type createAccountReq struct {
	AgentDID string `json:"agentDid"`
}

type verifyAgentReq struct {
	AgentDID  string `json:"agentDid"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

type updateAgentReq struct {
	AgentDID       string `json:"agentDid"`
	NewDIDPubKey   string `json:"newDidPubKey"`
	SignTimestamp  string `json:"signTimestamp"`
	ProofSignature string `json:"proofSignature"`
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	acc := s.svc.CreateAccount(req.AgentDID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": acc})
}

func (s *Server) handleVerifyAgent(w http.ResponseWriter, r *http.Request) {
	var req verifyAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Message) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.VerifyAgentSignature(req.AgentDID, req.Message, req.Signature); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"verified": true}})
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	var req updateAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.NewDIDPubKey) == "" ||
		strings.TrimSpace(req.SignTimestamp) == "" ||
		strings.TrimSpace(req.ProofSignature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !isBase64Ed25519PubKey(req.NewDIDPubKey) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid did pub key"})
		return
	}
	if !s.validateSignatureTimestamp(req.SignTimestamp) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	proofMessage := buildUpdateKeyProofPayload(req.AgentDID, req.NewDIDPubKey, req.SignTimestamp)
	if err := s.svc.UpdateAgentPublicKey(req.AgentDID, req.NewDIDPubKey, proofMessage, req.ProofSignature); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAgentList(w http.ResponseWriter, r *http.Request) {
	userID := s.requestUserID(r)
	list := s.filterAgentsByOwner(s.svc.ListAgents(), userID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": list})
}

type rechargeReq struct {
	VAAccountID string `json:"vaAccountId"`
	VACardNo    string `json:"vaCardNo"`
	Currency    string `json:"currency"`
	Amount      string `json:"amount"`
}

func (s *Server) handleRecharge(w http.ResponseWriter, r *http.Request) {
	var req rechargeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	accountRef := strings.TrimSpace(req.VAAccountID)
	if accountRef == "" {
		accountRef = strings.TrimSpace(req.VACardNo)
	}
	if accountRef == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, accountRef) {
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Recharge(accountRef, req.Currency, req.Amount, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "recharge", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleRechargeList(w http.ResponseWriter, r *http.Request) {
	va := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if va != "" && !s.ensureAccountOwnedByRef(w, r, va) {
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	list := s.svc.ListRecharges(va, limit)
	if va == "" {
		list = s.filterRechargesByOwner(list, s.requestUserID(r))
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": list})
}

type authorizeReq struct {
	AgentDID    string   `json:"agentDid"`
	SingleLimit string   `json:"singleLimit"`
	DailyLimit  string   `json:"dailyLimit"`
	Whitelist   []string `json:"whitelist"`
}

type freezeAuthorizeReq struct {
	AgentDID string `json:"agentDid"`
}

func (s *Server) handleAuthorizeSet(w http.ResponseWriter, r *http.Request) {
	var req authorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || !isPositiveDecimal(req.SingleLimit) || !isPositiveDecimal(req.DailyLimit) || len(req.Whitelist) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if err := s.svc.SetAuthorizeRule(req.AgentDID, req.SingleLimit, req.DailyLimit, req.Whitelist); err != nil {
		writeInternalError(w, r, "set authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_set", req.AgentDID, map[string]any{"singleLimit": req.SingleLimit, "dailyLimit": req.DailyLimit})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuthorizeUpdate(w http.ResponseWriter, r *http.Request) {
	var req authorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || !isPositiveDecimal(req.SingleLimit) || !isPositiveDecimal(req.DailyLimit) || len(req.Whitelist) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if err := s.svc.UpdateAuthorizeRule(req.AgentDID, req.SingleLimit, req.DailyLimit, req.Whitelist); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "update authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_update", req.AgentDID, map[string]any{"singleLimit": req.SingleLimit, "dailyLimit": req.DailyLimit})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuthorizeFreeze(w http.ResponseWriter, r *http.Request) {
	var req freezeAuthorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AgentDID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if err := s.svc.FreezeAuthorizeRule(req.AgentDID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "freeze authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_freeze", req.AgentDID, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuthorizeActivate(w http.ResponseWriter, r *http.Request) {
	var req freezeAuthorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AgentDID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if err := s.svc.ActivateAuthorizeRule(req.AgentDID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "activate authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_activate", req.AgentDID, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type payReq struct {
	PayerDID   string `json:"payerDid"`
	MerchantID string `json:"merchantId"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	Signature  string `json:"signature"`
}

type statusCallbackReq struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
}

type txActionReq struct {
	TransactionID string `json:"transactionId"`
}

type vaTopupConfigReq struct {
	AccountID        string `json:"accountId"`
	AutoTopupEnabled bool   `json:"autoTopupEnabled"`
	ThresholdAmount  string `json:"thresholdAmount"`
	TargetAmount     string `json:"targetAmount"`
}

type vaTransferReq struct {
	FromAccountID string `json:"fromAccountId"`
	ToAccountID   string `json:"toAccountId"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
}

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	var req payReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if req.PayerDID == "" || req.MerchantID == "" || req.Signature == "" || service.NormalizeCurrency(req.Currency) == "" || !isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.PayerDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	ipKey := clientIP(r)
	agentKey := "agent:" + req.PayerDID
	now := s.nowFn().UTC()
	if !s.ipRateLimiter.allow("ip:"+ipKey, now) || !s.agentRateLimiter.allow(agentKey, now) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"code": "PAY-010", "message": "rate limit exceeded"})
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(req.PayerDID)
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := buildPaySignaturePayload(req.PayerDID, req.MerchantID, req.Currency, req.Amount, idem, r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	log.Printf("requestId=%s pay request did=%s merchant=%s amount=%s ip=%s", getRequestID(r.Context()), maskDID(req.PayerDID), req.MerchantID, req.Amount, ipKey)
	resp, apiErr := s.svc.Pay(service.PayRequest{
		PayerDID:       req.PayerDID,
		MerchantID:     req.MerchantID,
		Currency:       req.Currency,
		Amount:         req.Amount,
		IdempotencyKey: idem,
		Signature:      req.Signature,
	})
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": resp})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("transactionId")
	tx, err := s.svc.QueryStatus(txID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-010", "message": "not found"})
		return
	}
	if !s.ensureAgentOwned(w, r, tx.PayerDID) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": tx})
}

func (s *Server) handleStatusCallback(w http.ResponseWriter, r *http.Request) {
	var req statusCallbackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	switch status {
	case "SETTLED":
		if err := s.svc.ResolveSettling(req.TransactionID, true); err != nil {
			writeInternalError(w, r, "resolve callback status", err)
			return
		}
	case "FAILED":
		if err := s.svc.ResolveSettling(req.TransactionID, false); err != nil {
			writeInternalError(w, r, "resolve callback status", err)
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid status"})
		return
	}
	log.Printf("requestId=%s callback resolved tx=%s status=%s", getRequestID(r.Context()), req.TransactionID, status)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleUnfreeze(w http.ResponseWriter, r *http.Request) {
	var req txActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Unfreeze(req.TransactionID, idem); err != nil {
		writeInternalError(w, r, "unfreeze", err)
		return
	}
	log.Printf("requestId=%s unfreeze tx=%s", getRequestID(r.Context()), req.TransactionID)
	s.appendAuditLog(r, "payment_unfreeze", req.TransactionID, map[string]any{"idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleRefund(w http.ResponseWriter, r *http.Request) {
	var req txActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Refund(req.TransactionID, idem); err != nil {
		writeInternalError(w, r, "refund", err)
		return
	}
	log.Printf("requestId=%s refund tx=%s", getRequestID(r.Context()), req.TransactionID)
	s.appendAuditLog(r, "payment_refund", req.TransactionID, map[string]any{"idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	va := r.URL.Query().Get("accountId")
	if !s.ensureAccountOwnedByRef(w, r, va) {
		return
	}
	currency := strings.TrimSpace(r.URL.Query().Get("currency"))
	balance, err := s.svc.BalanceByVA(va, currency)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-010", "message": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"balance": balance, "currency": service.NormalizeCurrency(currency)}})
}

func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request) {
	va := r.URL.Query().Get("accountId")
	if va == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, va) {
		return
	}
	currency := strings.TrimSpace(r.URL.Query().Get("currency"))
	ledger := s.svc.LedgerByVA(va, currency)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": ledger})
}

func (s *Server) handleOverviewMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.svc.OverviewMetrics()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": "PAY-010", "message": "query metrics failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": metrics})
}

func (s *Server) handleInterest(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, accountID) {
		return
	}
	quote, err := s.svc.QueryInterest(accountID)
	if err != nil {
		writeInternalError(w, r, "query interest", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": quote})
}

func (s *Server) handleVATopupConfigSet(w http.ResponseWriter, r *http.Request) {
	var req vaTopupConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AccountID) == "" ||
		!isNonNegativeDecimal(req.ThresholdAmount) ||
		!isPositiveDecimal(req.TargetAmount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, req.AccountID) {
		return
	}
	item, err := s.svc.SetVATopupConfig(req.AccountID, req.AutoTopupEnabled, req.ThresholdAmount, req.TargetAmount)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set va topup config", err)
		return
	}
	s.appendAuditLog(r, "va_topup_config_set", req.AccountID, map[string]any{
		"autoTopupEnabled": req.AutoTopupEnabled,
		"thresholdAmount":  req.ThresholdAmount,
		"targetAmount":     req.TargetAmount,
	})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleVATopupConfigGet(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, accountID) {
		return
	}
	item, err := s.svc.GetVATopupConfig(accountID)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "query va topup config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleVATransfer(w http.ResponseWriter, r *http.Request) {
	var req vaTransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.FromAccountID) == "" ||
		strings.TrimSpace(req.ToAccountID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, req.FromAccountID) || !s.ensureAccountOwnedByRef(w, r, req.ToAccountID) {
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.TransferVA(req.FromAccountID, req.ToAccountID, req.Currency, req.Amount, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "va transfer", err)
		return
	}
	s.appendAuditLog(r, "va_transfer", req.FromAccountID+"->"+req.ToAccountID, map[string]any{"amount": req.Amount, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleVATransferList(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID != "" && !s.ensureAccountOwnedByRef(w, r, accountID) {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	startTime := strings.TrimSpace(r.URL.Query().Get("startTime"))
	endTime := strings.TrimSpace(r.URL.Query().Get("endTime"))
	if startTime != "" {
		if _, err := time.Parse(time.RFC3339, startTime); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid startTime"})
			return
		}
	}
	if endTime != "" {
		if _, err := time.Parse(time.RFC3339, endTime); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid endTime"})
			return
		}
	}
	limit := 20
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items := s.svc.ListVATransfers(accountID, status, startTime, endTime, limit, offset)
	if accountID == "" {
		items = s.filterVATransfersByOwner(items, s.requestUserID(r))
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

type fundTransferReq struct {
	FromAccountID string `json:"fromAccountId"`
	ToAccountID   string `json:"toAccountId"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
}

func (s *Server) handleFundTransfer(w http.ResponseWriter, r *http.Request) {
	var req fundTransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.FromAccountID) == "" ||
		strings.TrimSpace(req.ToAccountID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, req.FromAccountID) || !s.ensureAccountOwnedByRef(w, r, req.ToAccountID) {
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.TransferFunds(req.FromAccountID, req.ToAccountID, req.Currency, req.Amount, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "fund transfer", err)
		return
	}
	log.Printf("requestId=%s fund_transfer from=%s to=%s amount=%s", getRequestID(r.Context()), req.FromAccountID, req.ToAccountID, req.Amount)
	s.appendAuditLog(r, "fund_transfer", req.FromAccountID+"->"+req.ToAccountID, map[string]any{"amount": req.Amount, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type fundWithdrawReq struct {
	VAAccountID     string `json:"vaAccountId"`
	Currency        string `json:"currency"`
	Amount          string `json:"amount"`
	Rail            string `json:"rail"`
	DestinationHint string `json:"destinationHint"`
}

func (s *Server) handleFundWithdraw(w http.ResponseWriter, r *http.Request) {
	var req fundWithdrawReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.VAAccountID) == "" ||
		service.NormalizeCurrency(req.Currency) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Rail) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, req.VAAccountID) {
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	rec, err := s.svc.WithdrawFunds(req.VAAccountID, req.Currency, req.Amount, req.Rail, req.DestinationHint, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "fund withdraw", err)
		return
	}
	log.Printf("requestId=%s fund_withdraw id=%s va=%s amount=%s rail=%s", getRequestID(r.Context()), rec.WithdrawID, rec.VAAccountID, req.Amount, rec.Rail)
	s.appendAuditLog(r, "fund_withdraw", rec.WithdrawID, map[string]any{"vaAccountId": rec.VAAccountID, "amount": req.Amount, "rail": rec.Rail, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type debitPreviewReq struct {
	AgentDID   string `json:"agentDid"`
	MerchantID string `json:"merchantId"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	Signature  string `json:"signature"`
}

func (s *Server) handleDebitPreview(w http.ResponseWriter, r *http.Request) {
	var req debitPreviewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		service.NormalizeCurrency(req.Currency) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(req.AgentDID)
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := []byte("debit_preview|" + req.AgentDID + "|" + req.MerchantID + "|" + req.Amount + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	pv, err := s.svc.DebitPreview(req.AgentDID, req.MerchantID, req.Amount)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "debit preview", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": pv})
}

type x402CheckReq struct {
	TransactionID string `json:"transactionId"`
}

func (s *Server) handleX402Check(w http.ResponseWriter, r *http.Request) {
	var req x402CheckReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	data, err := s.svc.CheckX402Settlement(req.TransactionID)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "x402 check", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": data})
}

type x402TransferReq struct {
	VAAccountID            string `json:"vaAccountId"`
	ToAddress              string `json:"toAddress"`
	Currency               string `json:"currency"`
	Amount                 string `json:"amount"`
	ReferenceTransactionID string `json:"referenceTransactionId"`
}

func (s *Server) handleX402Transfer(w http.ResponseWriter, r *http.Request) {
	var req x402TransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.VAAccountID) == "" ||
		strings.TrimSpace(req.ToAddress) == "" ||
		strings.TrimSpace(req.ReferenceTransactionID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAccountOwnedByRef(w, r, req.VAAccountID) {
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.TransferX402Outbound(req.VAAccountID, req.ToAddress, req.Currency, req.Amount, req.ReferenceTransactionID, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "x402 transfer", err)
		return
	}
	log.Printf("requestId=%s x402_outbound va=%s ref=%s amount=%s", getRequestID(r.Context()), req.VAAccountID, req.ReferenceTransactionID, req.Amount)
	s.appendAuditLog(r, "payment_x402_transfer", req.ReferenceTransactionID, map[string]any{"vaAccountId": req.VAAccountID, "toAddress": req.ToAddress, "amount": req.Amount, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type refundApplyReq struct {
	AgentDID      string `json:"agentDid"`
	TransactionID string `json:"transactionId"`
	Reason        string `json:"reason"`
	Signature     string `json:"signature"`
}

func (s *Server) handleRefundApply(w http.ResponseWriter, r *http.Request) {
	var req refundApplyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.TransactionID) == "" ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := []byte("refund_apply|" + strings.TrimSpace(req.TransactionID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.RefundApply(req.TransactionID, strings.TrimSpace(req.Reason), idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "refund apply", err)
		return
	}
	log.Printf("requestId=%s refund_apply tx=%s did=%s", getRequestID(r.Context()), req.TransactionID, req.AgentDID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type cardApplyReq struct {
	AgentDID    string `json:"agentDid"`
	VAAccountID string `json:"vaAccountId"`
	CreditLimit string `json:"creditLimit"`
}

func (s *Server) handleCardApply(w http.ResponseWriter, r *http.Request) {
	var req cardApplyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.VAAccountID) == "" ||
		!isPositiveDecimal(req.CreditLimit) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	rec, err := s.svc.ApplyVirtualCard(req.AgentDID, req.VAAccountID, req.CreditLimit, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "card apply", err)
		return
	}
	log.Printf("requestId=%s card_apply id=%s", getRequestID(r.Context()), rec.CardID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type cardPayReq struct {
	AgentDID   string `json:"agentDid"`
	CardID     string `json:"cardId"`
	MerchantID string `json:"merchantId"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	Signature  string `json:"signature"`
}

func (s *Server) handleCardPay(w http.ResponseWriter, r *http.Request) {
	var req cardPayReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.CardID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		service.NormalizeCurrency(req.Currency) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("card_pay|" + req.AgentDID + "|" + req.CardID + "|" + req.MerchantID + "|" + req.Amount + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	txID, err := s.svc.PayVirtualCard(req.AgentDID, req.CardID, req.MerchantID, req.Amount, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "card pay", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"transactionId": txID}})
}

type cardManageReq struct {
	CardID       string `json:"cardId"`
	Operation    string `json:"operation"`
	AdjustAmount string `json:"adjustAmount"`
}

func (s *Server) handleCardManage(w http.ResponseWriter, r *http.Request) {
	var req cardManageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.CardID) == "" ||
		strings.TrimSpace(req.Operation) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	rec, err := s.svc.ManageVirtualCard(req.CardID, req.Operation, req.AdjustAmount)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "card manage", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type riskTxnCheckReq struct {
	AgentDID      string `json:"agentDid"`
	MerchantID    string `json:"merchantId"`
	Amount        string `json:"amount"`
	TransactionID string `json:"transactionId"`
}

func (s *Server) handleRiskTransactionCheck(w http.ResponseWriter, r *http.Request) {
	var req riskTxnCheckReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	out, err := s.svc.RiskTransactionCheck(req.AgentDID, req.MerchantID, req.Amount, strings.TrimSpace(req.TransactionID))
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "risk transaction check", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": out})
}

type riskKYCReq struct {
	AgentDID           string `json:"agentDid"`
	DocumentReference  string `json:"documentReference"`
	Signature          string `json:"signature"`
}

func (s *Server) handleRiskKYCVerify(w http.ResponseWriter, r *http.Request) {
	var req riskKYCReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("kyc_verify|" + strings.TrimSpace(req.AgentDID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	st, err := s.svc.RiskKYCVerify(req.AgentDID, strings.TrimSpace(req.DocumentReference), idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "risk kyc verify", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": st})
}

func (s *Server) handleRiskAuditQuery(w http.ResponseWriter, r *http.Request) {
	agentDID := strings.TrimSpace(r.URL.Query().Get("agentDid"))
	if agentDID != "" && !s.ensureAgentOwned(w, r, agentDID) {
		return
	}
	merchantID := strings.TrimSpace(r.URL.Query().Get("merchantId"))
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	items := s.svc.RiskAuditQuery(agentDID, merchantID, limit, offset)
	if agentDID == "" {
		items = s.filterRiskAuditsByOwner(items, s.requestUserID(r))
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

type walletBindReq struct {
	AgentDID      string `json:"agentDid"`
	WalletAddress string `json:"walletAddress"`
	Label         string `json:"label"`
	Signature     string `json:"signature"`
}

func (s *Server) handleWalletBind(w http.ResponseWriter, r *http.Request) {
	var req walletBindReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.WalletAddress) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("wallet_bind|" + strings.TrimSpace(req.AgentDID) + "|" + strings.TrimSpace(req.WalletAddress) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.BindWallet(req.AgentDID, req.WalletAddress, req.Label); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "wallet bind", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type walletUnbindReq struct {
	AgentDID  string `json:"agentDid"`
	Signature string `json:"signature"`
}

func (s *Server) handleWalletUnbind(w http.ResponseWriter, r *http.Request) {
	var req walletUnbindReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("wallet_unbind|" + strings.TrimSpace(req.AgentDID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.UnbindWallet(req.AgentDID); err != nil {
		writeInternalError(w, r, "wallet unbind", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type sessionCreateReq struct {
	AgentDID    string `json:"agentDid"`
	TTLMinutes  int    `json:"ttlMinutes"`
	Signature   string `json:"signature"`
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var req sessionCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("session_create|" + strings.TrimSpace(req.AgentDID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	st, err := s.svc.CreateAuthSession(req.AgentDID, req.TTLMinutes, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "session create", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": st})
}

type sessionRevokeReq struct {
	AgentDID  string `json:"agentDid"`
	SessionID string `json:"sessionId"`
	Signature string `json:"signature"`
}

func (s *Server) handleSessionRevoke(w http.ResponseWriter, r *http.Request) {
	var req sessionRevokeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("session_revoke|" + strings.TrimSpace(req.AgentDID) + "|" + strings.TrimSpace(req.SessionID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.RevokeAuthSession(req.AgentDID, req.SessionID, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "session revoke", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type paymentSignRequestHTTP struct {
	AgentDID   string `json:"agentDid"`
	MerchantID string `json:"merchantId"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
	SessionID  string `json:"sessionId"`
	Signature  string `json:"signature"`
}

func (s *Server) handlePaymentSignRequest(w http.ResponseWriter, r *http.Request) {
	var req paymentSignRequestHTTP
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		service.NormalizeCurrency(req.Currency) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.AgentDID) {
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("sign_request|" + strings.TrimSpace(req.AgentDID) + "|" + strings.TrimSpace(req.MerchantID) + "|" + strings.TrimSpace(req.Amount) + "|" + strings.TrimSpace(req.SessionID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	rec, err := s.svc.RequestPaymentSign(req.AgentDID, req.MerchantID, req.Amount, req.SessionID, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "payment sign request", err)
		return
	}
	log.Printf("requestId=%s payment_sign_request id=%s", getRequestID(r.Context()), rec.SignID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type paymentSignSubmitReq struct {
	SignID         string `json:"signId"`
	PayerDID       string `json:"payerDid"`
	MerchantID     string `json:"merchantId"`
	Currency       string `json:"currency"`
	Amount         string `json:"amount"`
	IdempotencyKey string `json:"idempotencyKey"`
	Signature      string `json:"signature"`
}

func (s *Server) handlePaymentSignSubmit(w http.ResponseWriter, r *http.Request) {
	var req paymentSignSubmitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.SignID) == "" ||
		strings.TrimSpace(req.PayerDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		service.NormalizeCurrency(req.Currency) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" ||
		strings.TrimSpace(req.IdempotencyKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	if !s.ensureAgentOwned(w, r, req.PayerDID) {
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.PayerDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := buildPaySignaturePayload(req.PayerDID, req.MerchantID, req.Currency, req.Amount, strings.TrimSpace(req.IdempotencyKey), r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	resp, apiErr := s.svc.SubmitSignedPayment(strings.TrimSpace(req.SignID), service.PayRequest{
		PayerDID:       strings.TrimSpace(req.PayerDID),
		MerchantID:     strings.TrimSpace(req.MerchantID),
		Currency:       strings.TrimSpace(req.Currency),
		Amount:         strings.TrimSpace(req.Amount),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		Signature:      strings.TrimSpace(req.Signature),
	})
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	log.Printf("requestId=%s payment_sign_submit signId=%s tx=%s", getRequestID(r.Context()), req.SignID, resp.TransactionID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": resp})
}

func (s *Server) handleAPIKeyList(w http.ResponseWriter, r *http.Request) {
	keys := s.svc.ListAPIKeys()
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": keys})
}

type createAPIKeyReq struct {
	Name string `json:"name"`
}

func (s *Server) handleAPIKeyCreate(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.CreateAPIKey(strings.TrimSpace(req.Name))
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "create api key", err)
		return
	}
	log.Printf("requestId=%s api key created id=%s", getRequestID(r.Context()), item.ID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.DeleteAPIKey(id); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "delete api key", err)
		return
	}
	log.Printf("requestId=%s api key deleted id=%s", getRequestID(r.Context()), id)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleWebhookList(w http.ResponseWriter, r *http.Request) {
	items := s.svc.ListWebhooks()
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

type createWebhookReq struct {
	URL   string `json:"url"`
	Event string `json:"event"`
}

func (s *Server) handleWebhookCreate(w http.ResponseWriter, r *http.Request) {
	var req createWebhookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.CreateWebhook(strings.TrimSpace(req.URL), strings.TrimSpace(req.Event))
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "create webhook", err)
		return
	}
	log.Printf("requestId=%s webhook created id=%s", getRequestID(r.Context()), item.ID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleWebhookDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.DeleteWebhook(id); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "delete webhook", err)
		return
	}
	log.Printf("requestId=%s webhook deleted id=%s", getRequestID(r.Context()), id)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleWebhookDeliveryList(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	event := strings.TrimSpace(r.URL.Query().Get("event"))
	webhookID := strings.TrimSpace(r.URL.Query().Get("webhookId"))
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items, err := s.svc.ListWebhookDeliveries(status, event, webhookID, limit, offset)
	if err != nil {
		writeInternalError(w, r, "list webhook deliveries", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

func (s *Server) handleWebhookDeliveryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.svc.WebhookDeliveryStats()
	if err != nil {
		writeInternalError(w, r, "query webhook delivery stats", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": stats})
}

type replayWebhookDeliveryReq struct {
	ID int64 `json:"id"`
}

type riskConfigSetReq struct {
	Enabled                bool              `json:"enabled"`
	SingleAmountLimit      string            `json:"singleAmountLimit"`
	SingleAmountLimitByCcy map[string]string `json:"singleAmountLimitByCurrency"`
	BlockedMerchants       []string          `json:"blockedMerchants"`
}

type channelRouteSetReq struct {
	MerchantID string `json:"merchantId"`
	Mode       string `json:"mode"`
}

type subscriptionChangeReq struct {
	PlanCode string `json:"planCode"`
}

type adminSubscriptionAdjustReq struct {
	UserID   string `json:"userId"`
	PlanCode string `json:"planCode"`
	Status   string `json:"status"`
}

func (s *Server) handleSubscriptionPlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.ListSubscriptionPlans()})
}

func (s *Server) handleSubscriptionCurrent(w http.ResponseWriter, r *http.Request) {
	userID := s.requestUserID(r)
	rec, err := s.svc.GetUserSubscription(userID)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "query current subscription", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

func (s *Server) handleBillingInvoices(w http.ResponseWriter, r *http.Request) {
	userID := s.requestUserID(r)
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.ListUserInvoices(userID, limit)})
}

func (s *Server) handleSubscriptionChange(w http.ResponseWriter, r *http.Request) {
	var req subscriptionChangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.PlanCode) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	userID := s.requestUserID(r)
	rec, err := s.svc.ChangeUserSubscription(userID, strings.TrimSpace(req.PlanCode), true)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "change subscription", err)
		return
	}
	s.appendAuditLog(r, "subscription_change", userID, map[string]any{"planCode": req.PlanCode})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

func (s *Server) handleAdminSubscriptions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.AdminListSubscriptions(limit, offset)})
}

func (s *Server) handleAdminSubscriptionAdjust(w http.ResponseWriter, r *http.Request) {
	var req adminSubscriptionAdjustReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.PlanCode) == "" || strings.TrimSpace(req.Status) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	rec, err := s.svc.AdminUpdateSubscription(strings.TrimSpace(req.UserID), strings.TrimSpace(req.PlanCode), strings.TrimSpace(req.Status), true)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "adjust subscription", err)
		return
	}
	s.appendAuditLog(r, "subscription_admin_adjust", req.UserID, map[string]any{"planCode": req.PlanCode, "status": req.Status})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

func (s *Server) handleWebhookDeliveryReplay(w http.ResponseWriter, r *http.Request) {
	var req replayWebhookDeliveryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.ReplayWebhookDelivery(req.ID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "replay webhook delivery", err)
		return
	}
	log.Printf("requestId=%s webhook delivery replay id=%d", getRequestID(r.Context()), req.ID)
	s.appendAuditLog(r, "webhook_delivery_replay", strconv.FormatInt(req.ID, 10), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuditLogList(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items := s.svc.ListAuditLogs(action, resource, limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

func (s *Server) handleRiskConfigGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.GetRiskConfig()})
}

func (s *Server) handleRiskConfigSet(w http.ResponseWriter, r *http.Request) {
	var req riskConfigSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !isPositiveDecimal(req.SingleAmountLimit) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.SetRiskConfig(req.Enabled, req.SingleAmountLimit, req.SingleAmountLimitByCcy, req.BlockedMerchants)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set risk config", err)
		return
	}
	s.appendAuditLog(r, "risk_config_set", "global", map[string]any{
		"enabled":           req.Enabled,
		"singleAmountLimit": req.SingleAmountLimit,
		"blockedCount":      len(req.BlockedMerchants),
	})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleChannelRouteList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.ListChannelRoutes()})
}

func (s *Server) handleChannelRouteSet(w http.ResponseWriter, r *http.Request) {
	var req channelRouteSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.SetChannelRoute(req.MerchantID, req.Mode)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set channel route", err)
		return
	}
	s.appendAuditLog(r, "channel_route_set", req.MerchantID, map[string]any{"mode": req.Mode})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleChannelRouteDelete(w http.ResponseWriter, r *http.Request) {
	merchantID := strings.TrimSpace(r.URL.Query().Get("merchantId"))
	if merchantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.DeleteChannelRoute(merchantID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "delete channel route", err)
		return
	}
	s.appendAuditLog(r, "channel_route_delete", merchantID, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "0",
		"data": map[string]any{
			"status":    "ok",
			"requestId": getRequestID(r.Context()),
			"time":      s.nowFn().UTC().Format(time.RFC3339),
		},
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := true
	readyErr := ""
	if s.readyCheck != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.readyCheck(ctx); err != nil {
			ready = false
			readyErr = "dependency_unavailable"
			log.Printf("requestId=%s readiness check failed: %v", getRequestID(r.Context()), err)
		}
	}
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}
	writeJSON(w, statusCode, map[string]any{
		"code": "0",
		"data": map[string]any{
			"ready":     ready,
			"error":     readyErr,
			"requestId": getRequestID(r.Context()),
			"time":      s.nowFn().UTC().Format(time.RFC3339),
		},
	})
}

func isPositiveDecimal(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	n, err := strconv.ParseFloat(v, 64)
	return err == nil && n > 0
}

func isNonNegativeDecimal(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	n, err := strconv.ParseFloat(v, 64)
	return err == nil && n >= 0
}

func (s *Server) validateSignatureTimestamp(raw string) bool {
	if raw == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	now := s.nowFn().UTC()
	if t.After(now.Add(s.signatureMaxSkew)) {
		return false
	}
	return now.Sub(t) <= s.signatureMaxSkew
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

func maskDID(did string) string {
	if len(did) <= 8 {
		return "***"
	}
	return did[:6] + "***" + did[len(did)-2:]
}

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if reqID == "" {
			reqID = "req_" + strconv.FormatInt(s.nowFn().UnixNano(), 10)
		}
		w.Header().Set("X-Request-Id", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func m6FundsEnabled() bool {
	v := strings.TrimSpace(os.Getenv("FEATURE_M6_FUNDS"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *Server) withM6Funds(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m6FundsEnabled() {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-011", "message": "feature disabled"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func m7CardRiskEnabled() bool {
	v := strings.TrimSpace(os.Getenv("FEATURE_M7_CARD_RISK"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *Server) withM7CardRisk(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m7CardRiskEnabled() {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-012", "message": "feature disabled"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func m8SelfHostedEnabled() bool {
	v := strings.TrimSpace(os.Getenv("FEATURE_M8_SELF_HOSTED"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *Server) withM8SelfHosted(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m8SelfHostedEnabled() {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-013", "message": "feature disabled"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isBase64Ed25519PubKey(raw string) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	return err == nil && len(decoded) == ed25519.PublicKeySize
}

func buildPaySignaturePayload(payerDID, merchantID, currency, amount, idemKey, ts string) []byte {
	return []byte(payerDID + "|" + merchantID + "|" + service.NormalizeCurrency(currency) + "|" + amount + "|" + idemKey + "|" + ts)
}

func buildUpdateKeyProofPayload(agentDID, newPubKey, ts string) string {
	return strings.TrimSpace(agentDID) + "|" + strings.TrimSpace(newPubKey) + "|" + strings.TrimSpace(ts)
}

func verifyDIDSignature(pubKeyBase64, signatureBase64 string, payload []byte) bool {
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

func (s *Server) withAdminAuth(next http.Handler) http.Handler {
	return s.withRoleAuth(false, next)
}

func (s *Server) withReadAuth(next http.Handler) http.Handler {
	return s.withRoleAuth(true, next)
}

func (s *Server) withRoleAuth(allowReadonly bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.authorizeByUserRole(r, allowReadonly) {
			next.ServeHTTP(w, r)
			return
		}
		adminConfigured := strings.TrimSpace(s.adminBearerToken) != ""
		readonlyConfigured := strings.TrimSpace(s.readonlyToken) != ""
		if !adminConfigured && !readonlyConfigured {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if adminConfigured && secureEqual(token, s.adminBearerToken) {
			next.ServeHTTP(w, r)
			return
		}
		if allowReadonly && readonlyConfigured && secureEqual(token, s.readonlyToken) {
			next.ServeHTTP(w, r)
			return
		}
		log.Printf("requestId=%s unauthorized path=%s ip=%s readonlyAllowed=%t", getRequestID(r.Context()), r.URL.Path, clientIP(r), allowReadonly)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "unauthorized"})
	})
}

func (s *Server) authorizeByUserRole(r *http.Request, allowReadonly bool) bool {
	role := strings.ToLower(strings.TrimSpace(r.Header.Get(userRoleHeader)))
	switch role {
	case "":
		return false
	case "admin", "operator":
		return true
	case "readonly":
		return allowReadonly
	default:
		return false
	}
}

func (s *Server) withCallbackToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(s.callbackToken) == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(r.Header.Get("X-Callback-Token"))
		if !secureEqual(token, s.callbackToken) {
			log.Printf("requestId=%s unauthorized callback path=%s ip=%s", getRequestID(r.Context()), r.URL.Path, clientIP(r))
			writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "unauthorized callback"})
			return
		}
		tsRaw := strings.TrimSpace(r.Header.Get("X-Callback-Timestamp"))
		if !s.validateSignatureTimestamp(tsRaw) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid callback timestamp"})
			return
		}
		idemKey := strings.TrimSpace(r.Header.Get("X-Callback-Idempotency-Key"))
		if idemKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing callback idempotency key"})
			return
		}
		nonce := strings.TrimSpace(r.Header.Get("X-Callback-Nonce"))
		if nonce == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing callback nonce"})
			return
		}
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request body"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
		digest := callbackDigest(rawBody, tsRaw, nonce)
		if existed, same := s.callbackDedupe.Seen(idemKey, digest, s.nowFn().UTC()); existed {
			if same {
				writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]any{"code": "PAY-010", "message": "callback idempotency conflict"})
			return
		}
		if !s.callbackNonceLRU.allow("callback:"+nonce, s.nowFn().UTC()) {
			writeJSON(w, http.StatusConflict, map[string]any{"code": "PAY-010", "message": "replayed callback"})
			return
		}
		if strings.TrimSpace(s.callbackSignKey) != "" {
			version := strings.TrimSpace(r.Header.Get("X-Callback-Signature-Version"))
			if version != "v1" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "unsupported callback signature version"})
				return
			}
			var req statusCallbackReq
			if err := json.Unmarshal(rawBody, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
				return
			}
			expected := buildCallbackSignatureV1(s.callbackSignKey, req.TransactionID, req.Status, tsRaw, nonce, idemKey)
			got := strings.TrimSpace(r.Header.Get("X-Callback-Signature"))
			if !secureEqual(got, expected) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "invalid callback signature"})
				return
			}
		}
		sw := &statusCaptureWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(sw, r)
		if sw.statusCode >= 200 && sw.statusCode < 300 {
			s.callbackDedupe.Mark(idemKey, digest, s.nowFn().UTC())
		}
	})
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) requestUserID(r *http.Request) string {
	userID := strings.TrimSpace(r.Header.Get(userIDHeader))
	if userID != "" {
		return userID
	}
	// Backward-compatible fallback for direct backend calls/tests.
	return "system"
}

func (s *Server) bindAgentOwner(agentDID, userID string) {
	agent := strings.TrimSpace(agentDID)
	user := strings.TrimSpace(userID)
	if agent == "" || user == "" {
		return
	}
	_ = s.svc.BindAgentOwner(agent, user)
	s.agentOwnersMu.Lock()
	s.agentOwners[agent] = user
	s.agentOwnersMu.Unlock()
}

func (s *Server) ensureAgentOwned(w http.ResponseWriter, r *http.Request, agentDID string) bool {
	userID := s.requestUserID(r)
	agent := strings.TrimSpace(agentDID)
	if agent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return false
	}
	if userID == "" {
		// Backward compatibility for legacy callers/tests without user header.
		s.agentOwnersMu.RLock()
		_, hasAnyOwner := s.agentOwners[agent]
		s.agentOwnersMu.RUnlock()
		if !hasAnyOwner {
			return true
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "unauthorized"})
		return false
	}
	s.agentOwnersMu.RLock()
	owner, ok := s.agentOwners[agent]
	s.agentOwnersMu.RUnlock()
	if !ok {
		if dbOwner, exists, err := s.svc.AgentOwner(agent); err == nil && exists && strings.TrimSpace(dbOwner) != "" {
			owner = strings.TrimSpace(dbOwner)
			ok = true
			s.agentOwnersMu.Lock()
			s.agentOwners[agent] = owner
			s.agentOwnersMu.Unlock()
		}
	}
	if !ok {
		// Compatibility path: bind unowned pre-existing agents to current user.
		s.bindAgentOwner(agent, userID)
		return true
	}
	if owner != userID {
		writeJSON(w, http.StatusForbidden, map[string]any{"code": "PAY-010", "message": "forbidden"})
		return false
	}
	return true
}

func (s *Server) filterAgentsByOwner(items []service.AgentSummary, userID string) []service.AgentSummary {
	if strings.TrimSpace(userID) == "" {
		return []service.AgentSummary{}
	}
	out := make([]service.AgentSummary, 0, len(items))
	s.agentOwnersMu.RLock()
	defer s.agentOwnersMu.RUnlock()
	for _, item := range items {
		if s.agentOwners[strings.TrimSpace(item.AgentDID)] == userID {
			out = append(out, item)
		}
	}
	return out
}

func (s *Server) accountToAgentDID(accountRef string) string {
	ref := strings.TrimSpace(accountRef)
	if ref == "" {
		return ""
	}
	for _, item := range s.svc.ListAgents() {
		if strings.TrimSpace(item.VAAccountID) == ref || strings.TrimSpace(item.VACardNo) == ref {
			return strings.TrimSpace(item.AgentDID)
		}
	}
	return ""
}

func (s *Server) ensureAccountOwnedByRef(w http.ResponseWriter, r *http.Request, accountRef string) bool {
	agentDID := s.accountToAgentDID(accountRef)
	if agentDID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "account not found"})
		return false
	}
	return s.ensureAgentOwned(w, r, agentDID)
}

func (s *Server) filterRechargesByOwner(items []service.RechargeOrder, userID string) []service.RechargeOrder {
	if strings.TrimSpace(userID) == "" {
		return []service.RechargeOrder{}
	}
	out := make([]service.RechargeOrder, 0, len(items))
	for _, item := range items {
		if agent := s.accountToAgentDID(item.VAAccountID); agent != "" && s.isAgentOwnedByUser(agent, userID) {
			out = append(out, item)
		}
	}
	return out
}

func (s *Server) filterVATransfersByOwner(items []service.VATransferRecord, userID string) []service.VATransferRecord {
	if strings.TrimSpace(userID) == "" {
		return []service.VATransferRecord{}
	}
	out := make([]service.VATransferRecord, 0, len(items))
	for _, item := range items {
		fromAgent := s.accountToAgentDID(item.FromAccountID)
		toAgent := s.accountToAgentDID(item.ToAccountID)
		if (fromAgent != "" && s.isAgentOwnedByUser(fromAgent, userID)) || (toAgent != "" && s.isAgentOwnedByUser(toAgent, userID)) {
			out = append(out, item)
		}
	}
	return out
}

func (s *Server) filterRiskAuditsByOwner(items []service.RiskAuditEntry, userID string) []service.RiskAuditEntry {
	if strings.TrimSpace(userID) == "" {
		return []service.RiskAuditEntry{}
	}
	out := make([]service.RiskAuditEntry, 0, len(items))
	for _, item := range items {
		if s.isAgentOwnedByUser(strings.TrimSpace(item.AgentDID), userID) {
			out = append(out, item)
		}
	}
	return out
}

func (s *Server) isAgentOwnedByUser(agentDID, userID string) bool {
	agent := strings.TrimSpace(agentDID)
	if agent == "" || strings.TrimSpace(userID) == "" {
		return false
	}
	s.agentOwnersMu.RLock()
	owner, ok := s.agentOwners[agent]
	s.agentOwnersMu.RUnlock()
	if ok {
		return owner == userID
	}
	// Compatibility path: if this agent has not been bound yet, treat as not visible in list filters.
	return false
}

func (s *Server) appendAuditLog(r *http.Request, action string, resource string, detail map[string]any) {
	role := "admin"
	actor := "system"
	if token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); token != "" {
		if s.readonlyToken != "" && secureEqual(token, s.readonlyToken) {
			role = "readonly"
		}
		if len(token) > 8 {
			actor = token[:4] + "***" + token[len(token)-2:]
		} else {
			actor = "***"
		}
	}
	if err := s.svc.AppendAuditLog(actor, role, action, resource, getRequestID(r.Context()), detail); err != nil {
		log.Printf("requestId=%s append audit failed: %v", getRequestID(r.Context()), err)
	}
}

func buildCallbackSignatureV1(secret, transactionID, status, ts, nonce, idemKey string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	payload := strings.TrimSpace(transactionID) + "|" + strings.ToUpper(strings.TrimSpace(status)) + "|" + strings.TrimSpace(ts) + "|" + strings.TrimSpace(nonce) + "|" + strings.TrimSpace(idemKey)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

type statusCaptureWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

type callbackDedupeStore struct {
	mu      sync.Mutex
	window  time.Duration
	records map[string]callbackRecord
}

type callbackRecord struct {
	digest string
	seenAt time.Time
}

func newCallbackDedupeStore(window time.Duration) *callbackDedupeStore {
	return &callbackDedupeStore{
		window:  window,
		records: map[string]callbackRecord{},
	}
}

func (s *callbackDedupeStore) Seen(idemKey, digest string, now time.Time) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(now)
	record, ok := s.records[idemKey]
	if !ok {
		return false, false
	}
	return true, secureEqual(record.digest, digest)
}

func (s *callbackDedupeStore) Mark(idemKey, digest string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(now)
	s.records[idemKey] = callbackRecord{digest: digest, seenAt: now}
}

func (s *callbackDedupeStore) prune(now time.Time) {
	expireBefore := now.Add(-s.window)
	for key, record := range s.records {
		if record.seenAt.Before(expireBefore) {
			delete(s.records, key)
		}
	}
}

func callbackDigest(body []byte, ts, nonce string) string {
	sum := sha256.Sum256([]byte(string(body) + "|" + strings.TrimSpace(ts) + "|" + strings.TrimSpace(nonce)))
	return hex.EncodeToString(sum[:])
}


type stablecoinConfigSetReq struct {
	Currency         string `json:"currency"`
	Enabled          bool   `json:"enabled"`
	ChainID          string `json:"chainId"`
	RPCURL           string `json:"rpcUrl"`
	TokenContract    string `json:"tokenContract"`
	Decimals         int    `json:"decimals"`
	HotWallet        string `json:"hotWallet"`
	MinConfirmations int    `json:"minConfirmations"`
	RiskThreshold    string `json:"riskThreshold"`
}

func (s *Server) handleStablecoinConfigList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.ListStablecoinConfigs()})
}

func (s *Server) handleStablecoinConfigSet(w http.ResponseWriter, r *http.Request) {
	var req stablecoinConfigSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || service.NormalizeCurrency(req.Currency) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	threshold := 0.0
	if strings.TrimSpace(req.RiskThreshold) != "" {
		v, err := strconv.ParseFloat(strings.TrimSpace(req.RiskThreshold), 64)
		if err != nil || v <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid riskThreshold"})
			return
		}
		threshold = v
	}
	item, err := s.svc.SetStablecoinConfig(service.StablecoinConfig{Currency: req.Currency, Enabled: req.Enabled, ChainID: req.ChainID, RPCURL: req.RPCURL, TokenContract: req.TokenContract, Decimals: req.Decimals, HotWallet: req.HotWallet, MinConfirmations: req.MinConfirmations, RiskThreshold: threshold})
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set stablecoin config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleRechargeConfirmQuery(w http.ResponseWriter, r *http.Request) {
	rechargeID := strings.TrimSpace(r.URL.Query().Get("rechargeId"))
	if rechargeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.GetRechargeConfirmation(rechargeID)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "recharge confirmation query", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}


type rechargeAddressReq struct {
	AgentDID string `json:"agentDid"`
	Currency string `json:"currency"`
	Mode     string `json:"mode"`
}

type rechargeCallbackReq struct {
	RechargeID    string `json:"rechargeId"`
	Currency      string `json:"currency"`
	Status        string `json:"status"`
	Confirmations int    `json:"confirmations"`
}

func (s *Server) handleRechargeAddressQuery(w http.ResponseWriter, r *http.Request) {
	agentDID := strings.TrimSpace(r.URL.Query().Get("agentDid"))
	currency := strings.TrimSpace(r.URL.Query().Get("currency"))
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	if agentDID == "" || service.NormalizeCurrency(currency) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.ensureAgentOwned(w, r, agentDID) {
		return
	}
	item, err := s.svc.GetRechargeAddress(agentDID, currency, mode)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "query recharge address", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleRechargeCallback(w http.ResponseWriter, r *http.Request) {
	var req rechargeCallbackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.RechargeID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.HandleRechargeCallback(service.RechargeCallback{RechargeID: req.RechargeID, Currency: req.Currency, Status: req.Status, Confirmations: req.Confirmations})
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "recharge callback", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}
