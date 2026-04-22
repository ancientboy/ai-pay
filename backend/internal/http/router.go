package http

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-pay-backend/internal/service"
)

type Server struct {
	svc              service.PaymentService
	nowFn            func() time.Time
	signatureMaxSkew time.Duration
	ipRateLimiter    *fixedWindowLimiter
	agentRateLimiter *fixedWindowLimiter
}

type contextKey string

const requestIDKey contextKey = "requestId"

func NewServer(svc service.PaymentService) *Server {
	return &Server{
		svc:              svc,
		nowFn:            time.Now,
		signatureMaxSkew: 5 * time.Minute,
		ipRateLimiter:    newFixedWindowLimiter(120, time.Minute),
		agentRateLimiter: newFixedWindowLimiter(60, time.Minute),
	}
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
	mux.HandleFunc("GET /payment/status/query", s.handleStatus)
	mux.HandleFunc("GET /account/balance/query", s.handleBalance)
	mux.HandleFunc("GET /account/ledger/query", s.handleLedger)
	mux.HandleFunc("GET /metrics/overview", s.handleOverviewMetrics)
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

type registerAgentReq struct {
	AgentDID string `json:"agentDid"`
}

func (s *Server) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req registerAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	agent := s.svc.RegisterAgent(req.AgentDID)
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
	Amount      string `json:"amount"`
}

func (s *Server) handleRecharge(w http.ResponseWriter, r *http.Request) {
	var req rechargeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VAAccountID == "" || !isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.Recharge(req.VAAccountID, req.Amount); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": err.Error()})
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
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": err.Error()})
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
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "0",
		"data": map[string]any{
			"ready":     true,
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
