package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ai-pay-backend/internal/service"
)

func TestPayRejectsExpiredSignatureTimestamp(t *testing.T) {
	svc := seedServiceForPay()
	now := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)
	server := NewServerForTest(svc, func() time.Time { return now }, 100, 100)

	body := map[string]string{
		"payerDid":   "did:gusd:agent:test_http",
		"merchantId": "m1",
		"amount":     "1",
		"signature":  "sig",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-http-1")
	req.Header.Set("X-Sign-Timestamp", now.Add(-10*time.Minute).Format(time.RFC3339))
	rr := httptest.NewRecorder()

	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
	}
}

func TestPayRejectsRateLimit(t *testing.T) {
	svc := seedServiceForPay()
	now := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)
	server := NewServerForTest(svc, func() time.Time { return now }, 1, 1)

	first := performPay(server, "idem-http-2")
	if first != http.StatusOK {
		t.Fatalf("expected first 200 got %d", first)
	}
	second := performPay(server, "idem-http-3")
	if second != http.StatusTooManyRequests {
		t.Fatalf("expected second 429 got %d", second)
	}
}

func TestAuthorizeSetValidatesFields(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	body := map[string]any{
		"agentDid":    "did:gusd:agent:test_http2",
		"singleLimit": "10",
		"dailyLimit":  "100",
		"whitelist":   []string{},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/authorize/payment/set", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
	}
}

func TestRequestIDHeaderInjected(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	req := httptest.NewRequest(http.MethodGet, "/account/ledger/query?accountId=va_1", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Header().Get("X-Request-Id") == "" {
		t.Fatalf("expected X-Request-Id header")
	}
}

func TestHealthAndReady(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	handler := server.Routes()

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthResp := httptest.NewRecorder()
	handler.ServeHTTP(healthResp, healthReq)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health expected 200 got %d", healthResp.Code)
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyResp := httptest.NewRecorder()
	handler.ServeHTTP(readyResp, readyReq)
	if readyResp.Code != http.StatusOK {
		t.Fatalf("ready expected 200 got %d", readyResp.Code)
	}
}

func TestOverviewMetrics(t *testing.T) {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:m1")
	acc := svc.CreateAccount("did:gusd:agent:m1")
	_ = svc.Recharge(acc.VAAccountID, "100")
	_ = svc.SetAuthorizeRule("did:gusd:agent:m1", "20", "100", []string{"m1"})
	_, _ = svc.Pay(service.PayRequest{
		PayerDID:       "did:gusd:agent:m1",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-metrics-1",
		Signature:      "sig",
	})

	server := NewServerForTest(svc, time.Now, 100, 100)
	req := httptest.NewRequest(http.MethodGet, "/metrics/overview", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("overview expected 200 got %d", rr.Code)
	}
}

func TestAgentAndRechargeList(t *testing.T) {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:list1")
	acc := svc.CreateAccount("did:gusd:agent:list1")
	_ = svc.Recharge(acc.VAAccountID, "12")

	server := NewServerForTest(svc, time.Now, 100, 100)

	agentReq := httptest.NewRequest(http.MethodGet, "/agent/list", nil)
	agentResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(agentResp, agentReq)
	if agentResp.Code != http.StatusOK {
		t.Fatalf("agent list expected 200 got %d", agentResp.Code)
	}

	rechargeReq := httptest.NewRequest(http.MethodGet, "/fund/recharge/list?accountId="+acc.VAAccountID, nil)
	rechargeResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(rechargeResp, rechargeReq)
	if rechargeResp.Code != http.StatusOK {
		t.Fatalf("recharge list expected 200 got %d", rechargeResp.Code)
	}
}

func seedServiceForPay() *service.Service {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:test_http")
	acc := svc.CreateAccount("did:gusd:agent:test_http")
	_ = svc.Recharge(acc.VAAccountID, "100")
	_ = svc.SetAuthorizeRule("did:gusd:agent:test_http", "50", "200", []string{"m1"})
	return svc
}

func performPay(server *Server, idem string) int {
	body := map[string]string{
		"payerDid":   "did:gusd:agent:test_http",
		"merchantId": "m1",
		"amount":     "1",
		"signature":  "sig",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idem)
	req.Header.Set("X-Sign-Timestamp", time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC).Format(time.RFC3339))
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	return rr.Code
}
