package http

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ai-pay-backend/internal/service"
)

var testPayPrivateKey ed25519.PrivateKey

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

func TestPayRejectsInvalidDIDSignature(t *testing.T) {
	svc := seedServiceForPay()
	now := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)
	server := NewServerForTest(svc, func() time.Time { return now }, 100, 100)

	body := map[string]string{
		"payerDid":   "did:gusd:agent:test_http",
		"merchantId": "m1",
		"amount":     "1",
		"signature":  "invalid-signature",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-invalid-sign-1")
	req.Header.Set("X-Sign-Timestamp", now.Format(time.RFC3339))
	rr := httptest.NewRecorder()

	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
	}
}

func TestPayRejectsWhenAgentPublicKeyMissing(t *testing.T) {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:nokey")
	acc := svc.CreateAccount("did:gusd:agent:nokey")
	_ = svc.Recharge(acc.VAAccountID, "20", "rch-http-nokey-1")
	_ = svc.SetAuthorizeRule("did:gusd:agent:nokey", "20", "100", []string{"m1"})
	now := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC)
	server := NewServerForTest(svc, func() time.Time { return now }, 100, 100)

	body := map[string]string{
		"payerDid":   "did:gusd:agent:nokey",
		"merchantId": "m1",
		"amount":     "1",
		"signature":  "invalid",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-no-key-1")
	req.Header.Set("X-Sign-Timestamp", now.Format(time.RFC3339))
	rr := httptest.NewRecorder()

	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
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
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-http-overview-1")
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
	_ = svc.Recharge(acc.VAAccountID, "12", "rch-http-list-1")

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

func TestRechargeByVACardNo(t *testing.T) {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:card-http")
	acc := svc.CreateAccount("did:gusd:agent:card-http")
	server := NewServerForTest(svc, time.Now, 100, 100)

	body := map[string]string{
		"vaCardNo": acc.VACardNo,
		"amount":   "15",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/fund/recharge", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "rch-http-card-1")
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected recharge by card 200 got %d", rr.Code)
	}

	balanceReq := httptest.NewRequest(http.MethodGet, "/account/balance/query?accountId="+acc.VAAccountID, nil)
	balanceResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(balanceResp, balanceReq)
	if balanceResp.Code != http.StatusOK {
		t.Fatalf("expected balance query 200 got %d", balanceResp.Code)
	}
}

func TestPayChannelTimeoutReturnsPAY007AndRollsBack(t *testing.T) {
	svc := service.New()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_ = svc.RegisterAgent("did:gusd:agent:timeout")
	_ = svc.SetAgentPublicKey("did:gusd:agent:timeout", base64.StdEncoding.EncodeToString(pub))
	acc := svc.CreateAccount("did:gusd:agent:timeout")
	_ = svc.Recharge(acc.VAAccountID, "50", "rch-http-timeout-1")
	_ = svc.SetAuthorizeRule("did:gusd:agent:timeout", "50", "200", []string{"m_fail"})
	server := NewServerForTest(svc, time.Now, 100, 100)

	idem := "idem-timeout-1"
	ts := time.Now().UTC().Format(time.RFC3339)
	signPayload := buildPaySignaturePayload("did:gusd:agent:timeout", "m_fail", "10", idem, ts)
	body := map[string]string{
		"payerDid":   "did:gusd:agent:timeout",
		"merchantId": "m_fail",
		"amount":     "10",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signPayload)),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idem)
	req.Header.Set("X-Sign-Timestamp", ts)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &payload)
	if payload["code"] != "PAY-007" {
		t.Fatalf("expected PAY-007 got %v", payload["code"])
	}

	balance, err := svc.BalanceByVA(acc.VAAccountID)
	if err != nil {
		t.Fatalf("query balance failed: %v", err)
	}
	if balance != 50 {
		t.Fatalf("expected rollback balance 50 got %v", balance)
	}
}

func TestStatusCallbackSettlesAsyncOrder(t *testing.T) {
	svc := service.New()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agent := "did:gusd:agent:async-callback"
	_ = svc.RegisterAgent(agent)
	_ = svc.SetAgentPublicKey(agent, base64.StdEncoding.EncodeToString(pub))
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "50", "rch-http-async-1")
	_ = svc.SetAuthorizeRule(agent, "50", "200", []string{"m_async"})
	server := NewServerForTest(svc, time.Now, 100, 100)

	idem := "idem-async-callback-1"
	ts := time.Now().UTC().Format(time.RFC3339)
	signPayload := buildPaySignaturePayload(agent, "m_async", "10", idem, ts)
	payBody := map[string]string{
		"payerDid":   agent,
		"merchantId": "m_async",
		"amount":     "10",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signPayload)),
	}
	payReq := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(mustJSONMap(t, payBody)))
	payReq.Header.Set("Content-Type", "application/json")
	payReq.Header.Set("Idempotency-Key", idem)
	payReq.Header.Set("X-Sign-Timestamp", ts)
	payResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(payResp, payReq)
	if payResp.Code != http.StatusOK {
		t.Fatalf("pay expected 200 got %d", payResp.Code)
	}
	var payPayload map[string]any
	_ = json.Unmarshal(payResp.Body.Bytes(), &payPayload)
	data := payPayload["data"].(map[string]any)
	txID := data["transactionId"].(string)

	callbackBody := mustJSONMap(t, map[string]string{
		"transactionId": txID,
		"status":        "SETTLED",
	})
	callbackReq := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(callbackBody))
	callbackReq.Header.Set("Content-Type", "application/json")
	callbackResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(callbackResp, callbackReq)
	if callbackResp.Code != http.StatusOK {
		t.Fatalf("callback expected 200 got %d", callbackResp.Code)
	}
	tx, err := svc.QueryStatus(txID)
	if err != nil {
		t.Fatalf("query status failed: %v", err)
	}
	if tx.Status != "SETTLED" {
		t.Fatalf("expected SETTLED got %s", tx.Status)
	}
}

func TestUnfreezeEndpoint(t *testing.T) {
	svc := service.New()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agent := "did:gusd:agent:unfreeze-http"
	_ = svc.RegisterAgent(agent)
	_ = svc.SetAgentPublicKey(agent, base64.StdEncoding.EncodeToString(pub))
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "60", "rch-http-unfreeze-1")
	_ = svc.SetAuthorizeRule(agent, "60", "200", []string{"m_async"})
	server := NewServerForTest(svc, time.Now, 100, 100)

	idem := "idem-http-unfreeze-pay-1"
	ts := time.Now().UTC().Format(time.RFC3339)
	signPayload := buildPaySignaturePayload(agent, "m_async", "10", idem, ts)
	payBody := map[string]string{
		"payerDid":   agent,
		"merchantId": "m_async",
		"amount":     "10",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signPayload)),
	}
	payReq := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(mustJSONMap(t, payBody)))
	payReq.Header.Set("Content-Type", "application/json")
	payReq.Header.Set("Idempotency-Key", idem)
	payReq.Header.Set("X-Sign-Timestamp", ts)
	payResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(payResp, payReq)
	if payResp.Code != http.StatusOK {
		t.Fatalf("pay expected 200 got %d", payResp.Code)
	}
	var payPayload map[string]any
	_ = json.Unmarshal(payResp.Body.Bytes(), &payPayload)
	txID := payPayload["data"].(map[string]any)["transactionId"].(string)

	unfreezeReq := httptest.NewRequest(http.MethodPost, "/payment/unfreeze", bytes.NewReader(mustJSONMap(t, map[string]string{"transactionId": txID})))
	unfreezeReq.Header.Set("Content-Type", "application/json")
	unfreezeReq.Header.Set("Idempotency-Key", "idem-http-unfreeze-action-1")
	unfreezeResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(unfreezeResp, unfreezeReq)
	if unfreezeResp.Code != http.StatusOK {
		t.Fatalf("unfreeze expected 200 got %d", unfreezeResp.Code)
	}
}

func TestRefundEndpoint(t *testing.T) {
	svc := seedServiceForPay()
	server := NewServerForTest(svc, time.Now, 100, 100)
	idem := "idem-http-refund-pay-1"
	var txID string
	{
		body := map[string]string{
			"payerDid":   "did:gusd:agent:test_http",
			"merchantId": "m1",
			"amount":     "1",
		}
		ts := time.Now().UTC().Format(time.RFC3339)
		payload := buildPaySignaturePayload(body["payerDid"], body["merchantId"], body["amount"], idem, ts)
		body["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(testPayPrivateKey, payload))
		req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(mustJSONMap(t, body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idem)
		req.Header.Set("X-Sign-Timestamp", ts)
		resp := httptest.NewRecorder()
		server.Routes().ServeHTTP(resp, req)
		var payPayload map[string]any
		_ = json.Unmarshal(resp.Body.Bytes(), &payPayload)
		txID = payPayload["data"].(map[string]any)["transactionId"].(string)
	}

	refundReq := httptest.NewRequest(http.MethodPost, "/payment/refund", bytes.NewReader(mustJSONMap(t, map[string]string{"transactionId": txID})))
	refundReq.Header.Set("Content-Type", "application/json")
	refundReq.Header.Set("Idempotency-Key", "idem-http-refund-action-1")
	refundResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(refundResp, refundReq)
	if refundResp.Code != http.StatusOK {
		t.Fatalf("refund expected 200 got %d", refundResp.Code)
	}
}

func seedServiceForPay() *service.Service {
	svc := service.New()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	testPayPrivateKey = priv
	_ = svc.RegisterAgent("did:gusd:agent:test_http")
	_ = svc.SetAgentPublicKey("did:gusd:agent:test_http", base64.StdEncoding.EncodeToString(pub))
	acc := svc.CreateAccount("did:gusd:agent:test_http")
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-http-seed-1")
	_ = svc.SetAuthorizeRule("did:gusd:agent:test_http", "50", "200", []string{"m1"})
	return svc
}

func performPay(server *Server, idem string) int {
	body := map[string]string{
		"payerDid":   "did:gusd:agent:test_http",
		"merchantId": "m1",
		"amount":     "1",
	}
	ts := time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	payload := buildPaySignaturePayload(body["payerDid"], body["merchantId"], body["amount"], idem, ts)
	body["signature"] = base64.StdEncoding.EncodeToString(ed25519.Sign(testPayPrivateKey, payload))
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idem)
	req.Header.Set("X-Sign-Timestamp", ts)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	return rr.Code
}

func mustJSONMap(t *testing.T, body map[string]string) []byte {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	return b
}
