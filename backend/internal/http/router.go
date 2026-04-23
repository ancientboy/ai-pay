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
}

type contextKey string

const requestIDKey contextKey = "requestId"

func NewServer(svc service.PaymentService) *Server {
	return &Server{
		svc:              svc,
		nowFn:            time.Now,
		signatureMaxSkew: 5 * time.Minute,
		callbackNonceLRU: newFixedWindowLimiter(1, 10*time.Minute),
		callbackDedupe:   newCallbackDedupeStore(10 * time.Minute),
		ipRateLimiter:    newFixedWindowLimiter(120, time.Minute),
		agentRateLimiter: newFixedWindowLimiter(60, time.Minute),
	}
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
	mux.HandleFunc("POST /account/create", s.handleCreateAccount)
	mux.HandleFunc("GET /agent/list", s.handleAgentList)
	mux.HandleFunc("POST /fund/recharge", s.handleRecharge)
	mux.HandleFunc("GET /fund/recharge/list", s.handleRechargeList)
	mux.HandleFunc("POST /authorize/payment/set", s.handleAuthorizeSet)
	mux.HandleFunc("POST /payment/x402/pay", s.handlePay)
	mux.Handle("POST /payment/status/callback", s.withCallbackToken(http.HandlerFunc(s.handleStatusCallback)))
	mux.Handle("POST /payment/unfreeze", s.withAdminAuth(http.HandlerFunc(s.handleUnfreeze)))
	mux.Handle("POST /payment/refund", s.withAdminAuth(http.HandlerFunc(s.handleRefund)))
	mux.HandleFunc("GET /payment/status/query", s.handleStatus)
	mux.HandleFunc("GET /account/balance/query", s.handleBalance)
	mux.HandleFunc("GET /account/ledger/query", s.handleLedger)
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

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	acc := s.svc.CreateAccount(req.AgentDID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": acc})
}

func (s *Server) handleAgentList(w http.ResponseWriter, r *http.Request) {
	list := s.svc.ListAgents()
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": list})
}

type rechargeReq struct {
	VAAccountID string `json:"vaAccountId"`
	VACardNo    string `json:"vaCardNo"`
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
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Recharge(accountRef, req.Amount, idem); err != nil {
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
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	list := s.svc.ListRecharges(va, limit)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": list})
}

type authorizeReq struct {
	AgentDID    string   `json:"agentDid"`
	SingleLimit string   `json:"singleLimit"`
	DailyLimit  string   `json:"dailyLimit"`
	Whitelist   []string `json:"whitelist"`
}

func (s *Server) handleAuthorizeSet(w http.ResponseWriter, r *http.Request) {
	var req authorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || !isPositiveDecimal(req.SingleLimit) || !isPositiveDecimal(req.DailyLimit) || len(req.Whitelist) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.SetAuthorizeRule(req.AgentDID, req.SingleLimit, req.DailyLimit, req.Whitelist); err != nil {
		writeInternalError(w, r, "set authorize rule", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type payReq struct {
	PayerDID   string `json:"payerDid"`
	MerchantID string `json:"merchantId"`
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

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	var req payReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if req.PayerDID == "" || req.MerchantID == "" || req.Signature == "" || !isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
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
	signPayload := buildPaySignaturePayload(req.PayerDID, req.MerchantID, req.Amount, idem, r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	log.Printf("requestId=%s pay request did=%s merchant=%s amount=%s ip=%s", getRequestID(r.Context()), maskDID(req.PayerDID), req.MerchantID, req.Amount, ipKey)
	resp, apiErr := s.svc.Pay(service.PayRequest{
		PayerDID:       req.PayerDID,
		MerchantID:     req.MerchantID,
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
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	va := r.URL.Query().Get("accountId")
	balance, err := s.svc.BalanceByVA(va)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-010", "message": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"balance": balance}})
}

func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request) {
	va := r.URL.Query().Get("accountId")
	if va == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	ledger := s.svc.LedgerByVA(va)
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

func isBase64Ed25519PubKey(raw string) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	return err == nil && len(decoded) == ed25519.PublicKeySize
}

func buildPaySignaturePayload(payerDID, merchantID, amount, idemKey, ts string) []byte {
	return []byte(payerDID + "|" + merchantID + "|" + amount + "|" + idemKey + "|" + ts)
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
