package http

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestVerifyAgentSignatureEndpoint(t *testing.T) {
	svc := service.New()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agent := "did:gusd:agent:verify-http"
	_ = svc.RegisterAgent(agent)
	_ = svc.SetAgentPublicKey(agent, base64.StdEncoding.EncodeToString(pub))
	server := NewServerForTest(svc, time.Now, 100, 100)
	msg := "verify-me"
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(msg)))

	okReq := httptest.NewRequest(
		http.MethodPost,
		"/agent/did/verify",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"agentDid":  agent,
			"message":   msg,
			"signature": sig,
		})),
	)
	okReq.Header.Set("Content-Type", "application/json")
	okResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(okResp, okReq)
	if okResp.Code != http.StatusOK {
		t.Fatalf("verify expected 200 got %d", okResp.Code)
	}

	badReq := httptest.NewRequest(
		http.MethodPost,
		"/agent/did/verify",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"agentDid":  agent,
			"message":   msg + "-tampered",
			"signature": sig,
		})),
	)
	badReq.Header.Set("Content-Type", "application/json")
	badResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(badResp, badReq)
	if badResp.Code != http.StatusBadRequest {
		t.Fatalf("verify expected 400 for bad signature got %d", badResp.Code)
	}
}

func TestUpdateAgentPublicKeyEndpoint(t *testing.T) {
	svc := service.New()
	oldPub, oldPriv, _ := ed25519.GenerateKey(rand.Reader)
	newPub, _, _ := ed25519.GenerateKey(rand.Reader)
	agent := "did:gusd:agent:update-http"
	_ = svc.RegisterAgent(agent)
	_ = svc.SetAgentPublicKey(agent, base64.StdEncoding.EncodeToString(oldPub))
	server := NewServerForTest(svc, time.Now, 100, 100)
	ts := time.Now().UTC().Format(time.RFC3339)
	proofMsg := buildUpdateKeyProofPayload(agent, base64.StdEncoding.EncodeToString(newPub), ts)
	proofSig := base64.StdEncoding.EncodeToString(ed25519.Sign(oldPriv, []byte(proofMsg)))

	okReq := httptest.NewRequest(
		http.MethodPost,
		"/agent/did/update",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"agentDid":       agent,
			"newDidPubKey":   base64.StdEncoding.EncodeToString(newPub),
			"signTimestamp":  ts,
			"proofSignature": proofSig,
		})),
	)
	okReq.Header.Set("Content-Type", "application/json")
	okResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(okResp, okReq)
	if okResp.Code != http.StatusOK {
		t.Fatalf("update expected 200 got %d", okResp.Code)
	}

	verifiedSig := base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(oldPriv), []byte("check")))
	verifyReq := httptest.NewRequest(
		http.MethodPost,
		"/agent/did/verify",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"agentDid":  agent,
			"message":   "check",
			"signature": verifiedSig,
		})),
	)
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(verifyResp, verifyReq)
	if verifyResp.Code == http.StatusOK {
		t.Fatalf("old key should be invalid after update")
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

func TestAuthorizeUpdateAndFreeze(t *testing.T) {
	svc := service.New()
	agent := "did:gusd:agent:auth-update"
	_ = svc.RegisterAgent(agent)
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-auth-update-1")
	_ = svc.SetAuthorizeRule(agent, "20", "100", []string{"m1"})
	server := NewServerForTest(svc, time.Now, 100, 100)

	updateBody := map[string]any{
		"agentDid":    agent,
		"singleLimit": "10",
		"dailyLimit":  "50",
		"whitelist":   []string{"m1"},
	}
	updateReq := httptest.NewRequest(http.MethodPost, "/authorize/payment/update", bytes.NewReader(mustJSONAny(t, updateBody)))
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("authorize update expected 200 got %d", updateResp.Code)
	}

	freezeReq := httptest.NewRequest(http.MethodPost, "/authorize/freeze", bytes.NewReader(mustJSONMap(t, map[string]string{
		"agentDid": agent,
	})))
	freezeReq.Header.Set("Content-Type", "application/json")
	freezeResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(freezeResp, freezeReq)
	if freezeResp.Code != http.StatusOK {
		t.Fatalf("authorize freeze expected 200 got %d", freezeResp.Code)
	}

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_ = svc.SetAgentPublicKey(agent, base64.StdEncoding.EncodeToString(pub))
	idem := "idem-auth-frozen-1"
	ts := time.Now().UTC().Format(time.RFC3339)
	signPayload := buildPaySignaturePayload(agent, "m1", "1", idem, ts)
	payBody := map[string]string{
		"payerDid":   agent,
		"merchantId": "m1",
		"amount":     "1",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signPayload)),
	}
	payReq := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(mustJSONMap(t, payBody)))
	payReq.Header.Set("Content-Type", "application/json")
	payReq.Header.Set("Idempotency-Key", idem)
	payReq.Header.Set("X-Sign-Timestamp", ts)
	payResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(payResp, payReq)
	if payResp.Code != http.StatusBadRequest {
		t.Fatalf("pay should be blocked after freeze, got %d", payResp.Code)
	}

	activateReq := httptest.NewRequest(http.MethodPost, "/authorize/activate", bytes.NewReader(mustJSONMap(t, map[string]string{
		"agentDid": agent,
	})))
	activateReq.Header.Set("Content-Type", "application/json")
	activateResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(activateResp, activateReq)
	if activateResp.Code != http.StatusOK {
		t.Fatalf("authorize activate expected 200 got %d", activateResp.Code)
	}

	idem2 := "idem-auth-active-2"
	ts2 := time.Now().UTC().Format(time.RFC3339)
	signPayload2 := buildPaySignaturePayload(agent, "m1", "1", idem2, ts2)
	payBody2 := map[string]string{
		"payerDid":   agent,
		"merchantId": "m1",
		"amount":     "1",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signPayload2)),
	}
	payReq2 := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(mustJSONMap(t, payBody2)))
	payReq2.Header.Set("Content-Type", "application/json")
	payReq2.Header.Set("Idempotency-Key", idem2)
	payReq2.Header.Set("X-Sign-Timestamp", ts2)
	payResp2 := httptest.NewRecorder()
	server.Routes().ServeHTTP(payResp2, payReq2)
	if payResp2.Code != http.StatusOK {
		t.Fatalf("pay should succeed after activate, got %d", payResp2.Code)
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

func TestReadyReturns503WhenDependencyCheckFails(t *testing.T) {
	svc := service.New()
	server := NewServerWithReadiness(svc, func(ctx context.Context) error {
		return errors.New("redis unavailable")
	})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()
	server.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready expected 503 got %d", rr.Code)
	}
	var payload struct {
		Code string `json:"code"`
		Data struct {
			Ready bool   `json:"ready"`
			Error string `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if payload.Code != "0" {
		t.Fatalf("expected code 0 got %s", payload.Code)
	}
	if payload.Data.Ready {
		t.Fatalf("expected ready=false when check fails")
	}
	if payload.Data.Error == "" {
		t.Fatalf("expected error detail in ready response")
	}
	if payload.Data.Error != "dependency_unavailable" {
		t.Fatalf("expected masked readiness error, got %s", payload.Data.Error)
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

func TestVAInterestAndTopupConfigEndpoints(t *testing.T) {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:week2-interest")
	acc := svc.CreateAccount("did:gusd:agent:week2-interest")
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-http-week2-1")
	server := NewServerForTest(svc, time.Now, 100, 100)

	interestReq := httptest.NewRequest(http.MethodGet, "/account/interest/query?accountId="+acc.VAAccountID, nil)
	interestResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(interestResp, interestReq)
	if interestResp.Code != http.StatusOK {
		t.Fatalf("interest query expected 200 got %d", interestResp.Code)
	}

	setBody := map[string]any{
		"accountId":        acc.VAAccountID,
		"autoTopupEnabled": true,
		"thresholdAmount":  "10",
		"targetAmount":     "50",
	}
	setReq := httptest.NewRequest(http.MethodPost, "/account/va/topup/config", bytes.NewReader(mustJSONAny(t, setBody)))
	setReq.Header.Set("Content-Type", "application/json")
	setResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(setResp, setReq)
	if setResp.Code != http.StatusOK {
		t.Fatalf("set topup config expected 200 got %d", setResp.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/account/va/topup/config?accountId="+acc.VAAccountID, nil)
	getResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get topup config expected 200 got %d", getResp.Code)
	}
}

func TestVATransferEndpointWithIdempotency(t *testing.T) {
	svc := service.New()
	_ = svc.RegisterAgent("did:gusd:agent:week2-transfer-a")
	_ = svc.RegisterAgent("did:gusd:agent:week2-transfer-b")
	accA := svc.CreateAccount("did:gusd:agent:week2-transfer-a")
	accB := svc.CreateAccount("did:gusd:agent:week2-transfer-b")
	_ = svc.Recharge(accA.VAAccountID, "20", "rch-http-week2-transfer-1")
	server := NewServerForTest(svc, time.Now, 100, 100)

	transferBody := map[string]string{
		"fromAccountId": accA.VAAccountID,
		"toAccountId":   accB.VAAccountID,
		"amount":        "5",
	}
	req1 := httptest.NewRequest(http.MethodPost, "/account/va/transfer", bytes.NewReader(mustJSONMap(t, transferBody)))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", "idem-http-week2-transfer-1")
	resp1 := httptest.NewRecorder()
	server.Routes().ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("va transfer expected 200 got %d", resp1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/account/va/transfer", bytes.NewReader(mustJSONMap(t, transferBody)))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "idem-http-week2-transfer-1")
	resp2 := httptest.NewRecorder()
	server.Routes().ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("va transfer idempotent retry expected 200 got %d", resp2.Code)
	}

	balA, _ := svc.BalanceByVA(accA.VAAccountID)
	balB, _ := svc.BalanceByVA(accB.VAAccountID)
	if balA != 15 {
		t.Fatalf("expected from balance 15 got %v", balA)
	}
	if balB != 5 {
		t.Fatalf("expected to balance 5 got %v", balB)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/account/va/transfer/list?accountId="+accA.VAAccountID+"&status=SETTLED&limit=10&offset=0", nil)
	listResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("va transfer list expected 200 got %d", listResp.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(listResp.Body.Bytes(), &payload)
	items, ok := payload["data"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected transfer list data")
	}
	first := items[0].(map[string]any)
	createdAt, _ := first["createdAt"].(string)
	if createdAt == "" {
		t.Fatalf("expected createdAt in transfer list item")
	}
	windowStart := time.Now().UTC().Add(-5 * time.Minute).Format(time.RFC3339)
	windowEnd := time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)
	timeReq := httptest.NewRequest(
		http.MethodGet,
		"/account/va/transfer/list?accountId="+accA.VAAccountID+"&status=SETTLED&startTime="+url.QueryEscape(windowStart)+"&endTime="+url.QueryEscape(windowEnd)+"&limit=10&offset=0",
		nil,
	)
	timeResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(timeResp, timeReq)
	if timeResp.Code != http.StatusOK {
		t.Fatalf("va transfer list with time window expected 200 got %d", timeResp.Code)
	}
	_ = json.Unmarshal(timeResp.Body.Bytes(), &payload)
	items, ok = payload["data"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected transfer list data in time window")
	}
}

func TestDeveloperAPIKeyAndWebhookCRUD(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	handler := server.Routes()

	createKeyReq := httptest.NewRequest(
		http.MethodPost,
		"/developer/api-keys",
		bytes.NewReader(mustJSONMap(t, map[string]string{"name": "default"})),
	)
	createKeyReq.Header.Set("Content-Type", "application/json")
	createKeyResp := httptest.NewRecorder()
	handler.ServeHTTP(createKeyResp, createKeyReq)
	if createKeyResp.Code != http.StatusOK {
		t.Fatalf("create api key expected 200 got %d", createKeyResp.Code)
	}

	var createdKeyPayload map[string]any
	_ = json.Unmarshal(createKeyResp.Body.Bytes(), &createdKeyPayload)
	keyData := createdKeyPayload["data"].(map[string]any)
	keyID := keyData["id"].(string)
	if keyID == "" {
		t.Fatalf("expected api key id")
	}

	listKeyReq := httptest.NewRequest(http.MethodGet, "/developer/api-keys", nil)
	listKeyResp := httptest.NewRecorder()
	handler.ServeHTTP(listKeyResp, listKeyReq)
	if listKeyResp.Code != http.StatusOK {
		t.Fatalf("list api keys expected 200 got %d", listKeyResp.Code)
	}
	var listedKeyPayload map[string]any
	_ = json.Unmarshal(listKeyResp.Body.Bytes(), &listedKeyPayload)
	listData := listedKeyPayload["data"].([]any)
	if len(listData) == 0 {
		t.Fatalf("expected api key list to contain data")
	}
	listedFirst := listData[0].(map[string]any)
	if listedFirst["key"] == keyData["key"] {
		t.Fatalf("expected listed api key to be masked")
	}

	deleteKeyReq := httptest.NewRequest(http.MethodDelete, "/developer/api-keys?id="+keyID, nil)
	deleteKeyResp := httptest.NewRecorder()
	handler.ServeHTTP(deleteKeyResp, deleteKeyReq)
	if deleteKeyResp.Code != http.StatusOK {
		t.Fatalf("delete api key expected 200 got %d", deleteKeyResp.Code)
	}

	createWebhookReq := httptest.NewRequest(
		http.MethodPost,
		"/developer/webhooks",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"url":   "https://example.com/hook",
			"event": "payment.settled",
		})),
	)
	createWebhookReq.Header.Set("Content-Type", "application/json")
	createWebhookResp := httptest.NewRecorder()
	handler.ServeHTTP(createWebhookResp, createWebhookReq)
	if createWebhookResp.Code != http.StatusOK {
		t.Fatalf("create webhook expected 200 got %d", createWebhookResp.Code)
	}
	createRefundWebhookReq := httptest.NewRequest(
		http.MethodPost,
		"/developer/webhooks",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"url":   "https://example.com/hook-refund",
			"event": "payment.refunded",
		})),
	)
	createRefundWebhookReq.Header.Set("Content-Type", "application/json")
	createRefundWebhookResp := httptest.NewRecorder()
	handler.ServeHTTP(createRefundWebhookResp, createRefundWebhookReq)
	if createRefundWebhookResp.Code != http.StatusOK {
		t.Fatalf("create refund webhook expected 200 got %d", createRefundWebhookResp.Code)
	}
	var createdRefundHookPayload map[string]any
	_ = json.Unmarshal(createRefundWebhookResp.Body.Bytes(), &createdRefundHookPayload)
	refundHookData := createdRefundHookPayload["data"].(map[string]any)
	refundHookID := refundHookData["id"].(string)

	blockedWebhookReq := httptest.NewRequest(
		http.MethodPost,
		"/developer/webhooks",
		bytes.NewReader(mustJSONMap(t, map[string]string{
			"url":   "http://127.0.0.1/callback",
			"event": "payment.settled",
		})),
	)
	blockedWebhookReq.Header.Set("Content-Type", "application/json")
	blockedWebhookResp := httptest.NewRecorder()
	handler.ServeHTTP(blockedWebhookResp, blockedWebhookReq)
	if blockedWebhookResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked webhook expected 400 got %d", blockedWebhookResp.Code)
	}

	var createdHookPayload map[string]any
	_ = json.Unmarshal(createWebhookResp.Body.Bytes(), &createdHookPayload)
	hookData := createdHookPayload["data"].(map[string]any)
	hookID := hookData["id"].(string)
	if hookID == "" {
		t.Fatalf("expected webhook id")
	}

	listWebhookReq := httptest.NewRequest(http.MethodGet, "/developer/webhooks", nil)
	listWebhookResp := httptest.NewRecorder()
	handler.ServeHTTP(listWebhookResp, listWebhookReq)
	if listWebhookResp.Code != http.StatusOK {
		t.Fatalf("list webhooks expected 200 got %d", listWebhookResp.Code)
	}

	deleteWebhookReq := httptest.NewRequest(http.MethodDelete, "/developer/webhooks?id="+hookID, nil)
	deleteWebhookResp := httptest.NewRecorder()
	handler.ServeHTTP(deleteWebhookResp, deleteWebhookReq)
	if deleteWebhookResp.Code != http.StatusOK {
		t.Fatalf("delete webhook expected 200 got %d", deleteWebhookResp.Code)
	}

	agent := "did:gusd:agent:delivery-http"
	_ = svc.RegisterAgent(agent)
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "20", "rch-http-delivery-1")
	_ = svc.SetAuthorizeRule(agent, "20", "100", []string{"m1"})
	payResp, payErr := svc.Pay(service.PayRequest{
		PayerDID:       agent,
		MerchantID:     "m1",
		Amount:         "2",
		IdempotencyKey: "idem-http-delivery-pay-1",
		Signature:      "sig",
	})
	if payErr != nil {
		t.Fatalf("seed pay failed: %v", payErr)
	}
	if err := svc.Refund(payResp.TransactionID, "idem-http-delivery-refund-1"); err != nil {
		t.Fatalf("seed refund failed: %v", err)
	}
	deliveryListReq := httptest.NewRequest(http.MethodGet, "/developer/webhook-deliveries?event=payment.refunded", nil)
	deliveryListResp := httptest.NewRecorder()
	handler.ServeHTTP(deliveryListResp, deliveryListReq)
	if deliveryListResp.Code != http.StatusOK {
		t.Fatalf("list webhook deliveries expected 200 got %d", deliveryListResp.Code)
	}
	var deliveryPayload map[string]any
	_ = json.Unmarshal(deliveryListResp.Body.Bytes(), &deliveryPayload)
	listed := deliveryPayload["data"].([]any)
	if len(listed) == 0 {
		t.Fatalf("expected webhook deliveries")
	}
	firstDelivery := listed[0].(map[string]any)
	deliveryID := int64(firstDelivery["id"].(float64))
	replayBody, _ := json.Marshal(map[string]any{"id": deliveryID})
	replayReq := httptest.NewRequest(http.MethodPost, "/developer/webhook-deliveries/replay", bytes.NewReader(replayBody))
	replayReq.Header.Set("Content-Type", "application/json")
	replayResp := httptest.NewRecorder()
	handler.ServeHTTP(replayResp, replayReq)
	if replayResp.Code != http.StatusOK {
		t.Fatalf("replay webhook delivery expected 200 got %d", replayResp.Code)
	}
	deliveryByWebhookReq := httptest.NewRequest(http.MethodGet, "/developer/webhook-deliveries?webhookId="+refundHookID+"&limit=1&offset=0", nil)
	deliveryByWebhookResp := httptest.NewRecorder()
	handler.ServeHTTP(deliveryByWebhookResp, deliveryByWebhookReq)
	if deliveryByWebhookResp.Code != http.StatusOK {
		t.Fatalf("delivery list by webhook id expected 200 got %d", deliveryByWebhookResp.Code)
	}
	deliveryStatsReq := httptest.NewRequest(http.MethodGet, "/developer/webhook-deliveries/stats", nil)
	deliveryStatsResp := httptest.NewRecorder()
	handler.ServeHTTP(deliveryStatsResp, deliveryStatsReq)
	if deliveryStatsResp.Code != http.StatusOK {
		t.Fatalf("delivery stats expected 200 got %d", deliveryStatsResp.Code)
	}
}

func TestDeveloperEndpointsRequireAdminTokenWhenConfigured(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	server.SetAdminBearerToken("test-admin-token")
	handler := server.Routes()

	noAuthReq := httptest.NewRequest(http.MethodGet, "/developer/api-keys", nil)
	noAuthResp := httptest.NewRecorder()
	handler.ServeHTTP(noAuthResp, noAuthReq)
	if noAuthResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without admin token, got %d", noAuthResp.Code)
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/developer/api-keys", nil)
	authedReq.Header.Set("Authorization", "Bearer test-admin-token")
	authedResp := httptest.NewRecorder()
	handler.ServeHTTP(authedResp, authedReq)
	if authedResp.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin token, got %d", authedResp.Code)
	}
}

func TestDeveloperReadEndpointsAllowReadonlyToken(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	server.SetAdminBearerToken("admin-token")
	server.SetReadonlyBearerToken("readonly-token")
	handler := server.Routes()

	readReq := httptest.NewRequest(http.MethodGet, "/developer/api-keys", nil)
	readReq.Header.Set("Authorization", "Bearer readonly-token")
	readResp := httptest.NewRecorder()
	handler.ServeHTTP(readResp, readReq)
	if readResp.Code != http.StatusOK {
		t.Fatalf("readonly token should access developer read endpoint, got %d", readResp.Code)
	}
	readDeliveryReq := httptest.NewRequest(http.MethodGet, "/developer/webhook-deliveries", nil)
	readDeliveryReq.Header.Set("Authorization", "Bearer readonly-token")
	readDeliveryResp := httptest.NewRecorder()
	handler.ServeHTTP(readDeliveryResp, readDeliveryReq)
	if readDeliveryResp.Code != http.StatusOK {
		t.Fatalf("readonly token should access delivery read endpoint, got %d", readDeliveryResp.Code)
	}
	readDeliveryStatsReq := httptest.NewRequest(http.MethodGet, "/developer/webhook-deliveries/stats", nil)
	readDeliveryStatsReq.Header.Set("Authorization", "Bearer readonly-token")
	readDeliveryStatsResp := httptest.NewRecorder()
	handler.ServeHTTP(readDeliveryStatsResp, readDeliveryStatsReq)
	if readDeliveryStatsResp.Code != http.StatusOK {
		t.Fatalf("readonly token should access delivery stats endpoint, got %d", readDeliveryStatsResp.Code)
	}

	writeReq := httptest.NewRequest(http.MethodPost, "/developer/api-keys", bytes.NewReader(mustJSONMap(t, map[string]string{"name": "r"})))
	writeReq.Header.Set("Content-Type", "application/json")
	writeReq.Header.Set("Authorization", "Bearer readonly-token")
	writeResp := httptest.NewRecorder()
	handler.ServeHTTP(writeResp, writeReq)
	if writeResp.Code != http.StatusUnauthorized {
		t.Fatalf("readonly token should not access developer write endpoint, got %d", writeResp.Code)
	}
}

func TestCallbackRequiresTokenWhenConfigured(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	server.SetCallbackToken("test-callback-token")
	handler := server.Routes()

	body := mustJSONMap(t, map[string]string{
		"transactionId": "tx_1",
		"status":        "SETTLED",
	})
	noTokenReq := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	noTokenReq.Header.Set("Content-Type", "application/json")
	noTokenResp := httptest.NewRecorder()
	handler.ServeHTTP(noTokenResp, noTokenReq)
	if noTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without callback token, got %d", noTokenResp.Code)
	}

	tokenReq := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	tokenReq.Header.Set("Content-Type", "application/json")
	tokenReq.Header.Set("X-Callback-Token", "test-callback-token")
	tokenReq.Header.Set("X-Callback-Timestamp", time.Now().UTC().Format(time.RFC3339))
	tokenReq.Header.Set("X-Callback-Nonce", "nonce-callback-1")
	tokenReq.Header.Set("X-Callback-Idempotency-Key", "cb-idem-1")
	tokenResp := httptest.NewRecorder()
	handler.ServeHTTP(tokenResp, tokenReq)
	// With token+timestamp+nonce it should pass auth and continue to business logic.
	if tokenResp.Code == http.StatusUnauthorized {
		t.Fatalf("expected callback auth checks to pass, got %d", tokenResp.Code)
	}
}

func TestCallbackRejectsReplayedNonce(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	server.SetCallbackToken("test-callback-token")
	handler := server.Routes()

	body := mustJSONMap(t, map[string]string{
		"transactionId": "tx_replay",
		"status":        "SETTLED",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Callback-Token", "test-callback-token")
	req1.Header.Set("X-Callback-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req1.Header.Set("X-Callback-Nonce", "nonce-replay-1")
	req1.Header.Set("X-Callback-Idempotency-Key", "cb-idem-replay-1")
	resp1 := httptest.NewRecorder()
	handler.ServeHTTP(resp1, req1)
	if resp1.Code == http.StatusUnauthorized {
		t.Fatalf("first callback should pass auth checks, got %d", resp1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Callback-Token", "test-callback-token")
	req2.Header.Set("X-Callback-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req2.Header.Set("X-Callback-Nonce", "nonce-replay-1")
	req2.Header.Set("X-Callback-Idempotency-Key", "cb-idem-replay-2")
	resp2 := httptest.NewRecorder()
	handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusConflict {
		t.Fatalf("expected replay callback 409, got %d", resp2.Code)
	}
}

func TestCallbackRequiresValidSignatureWhenConfigured(t *testing.T) {
	svc := service.New()
	server := NewServerForTest(svc, time.Now, 100, 100)
	server.SetCallbackToken("token-secure")
	server.SetCallbackSigningSecret("sign-secure")
	handler := server.Routes()

	body := mustJSONMap(t, map[string]string{
		"transactionId": "tx_sign",
		"status":        "SETTLED",
	})
	ts := time.Now().UTC().Format(time.RFC3339)
	nonce := "nonce-sign-1"

	reqWithoutSign := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	reqWithoutSign.Header.Set("Content-Type", "application/json")
	reqWithoutSign.Header.Set("X-Callback-Token", "token-secure")
	reqWithoutSign.Header.Set("X-Callback-Timestamp", ts)
	reqWithoutSign.Header.Set("X-Callback-Nonce", nonce)
	reqWithoutSign.Header.Set("X-Callback-Idempotency-Key", "cb-idem-sign-1")
	respWithoutSign := httptest.NewRecorder()
	handler.ServeHTTP(respWithoutSign, reqWithoutSign)
	if respWithoutSign.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without callback signature version, got %d", respWithoutSign.Code)
	}

	reqWithSign := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	reqWithSign.Header.Set("Content-Type", "application/json")
	reqWithSign.Header.Set("X-Callback-Token", "token-secure")
	reqWithSign.Header.Set("X-Callback-Timestamp", ts)
	reqWithSign.Header.Set("X-Callback-Nonce", "nonce-sign-2")
	reqWithSign.Header.Set("X-Callback-Idempotency-Key", "cb-idem-sign-2")
	reqWithSign.Header.Set("X-Callback-Signature-Version", "v1")
	reqWithSign.Header.Set("X-Callback-Signature", buildCallbackSignatureV1("sign-secure", "tx_sign", "SETTLED", ts, "nonce-sign-2", "cb-idem-sign-2"))
	respWithSign := httptest.NewRecorder()
	handler.ServeHTTP(respWithSign, reqWithSign)
	if respWithSign.Code == http.StatusUnauthorized {
		t.Fatalf("expected callback signature auth to pass with valid signature")
	}
}

func TestCallbackReplayByIdempotencyKeyReturnsOK(t *testing.T) {
	svc := service.New()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agent := "did:gusd:agent:callback-idem"
	_ = svc.RegisterAgent(agent)
	_ = svc.SetAgentPublicKey(agent, base64.StdEncoding.EncodeToString(pub))
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "50", "rch-http-cb-idem-1")
	_ = svc.SetAuthorizeRule(agent, "50", "200", []string{"m_async"})

	server := NewServerForTest(svc, time.Now, 100, 100)
	server.SetCallbackToken("token-retry")
	handler := server.Routes()

	idemPay := "idem-cb-idem-pay-1"
	tsPay := time.Now().UTC().Format(time.RFC3339)
	signPayload := buildPaySignaturePayload(agent, "m_async", "10", idemPay, tsPay)
	payBody := map[string]string{
		"payerDid":   agent,
		"merchantId": "m_async",
		"amount":     "10",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signPayload)),
	}
	payReq := httptest.NewRequest(http.MethodPost, "/payment/x402/pay", bytes.NewReader(mustJSONMap(t, payBody)))
	payReq.Header.Set("Content-Type", "application/json")
	payReq.Header.Set("Idempotency-Key", idemPay)
	payReq.Header.Set("X-Sign-Timestamp", tsPay)
	payResp := httptest.NewRecorder()
	handler.ServeHTTP(payResp, payReq)
	if payResp.Code != http.StatusOK {
		t.Fatalf("pay expected 200 got %d", payResp.Code)
	}
	var payPayload map[string]any
	_ = json.Unmarshal(payResp.Body.Bytes(), &payPayload)
	txID := payPayload["data"].(map[string]any)["transactionId"].(string)

	body := mustJSONMap(t, map[string]string{
		"transactionId": txID,
		"status":        "SETTLED",
	})
	ts := time.Now().UTC().Format(time.RFC3339)

	req1 := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Callback-Token", "token-retry")
	req1.Header.Set("X-Callback-Timestamp", ts)
	req1.Header.Set("X-Callback-Nonce", "nonce-retry-1")
	req1.Header.Set("X-Callback-Idempotency-Key", "cb-idem-retry-1")
	resp1 := httptest.NewRecorder()
	handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected first callback to succeed, got %d", resp1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/payment/status/callback", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Callback-Token", "token-retry")
	req2.Header.Set("X-Callback-Timestamp", ts)
	req2.Header.Set("X-Callback-Nonce", "nonce-retry-1")
	req2.Header.Set("X-Callback-Idempotency-Key", "cb-idem-retry-1")
	resp2 := httptest.NewRecorder()
	handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("expected callback retry by same idempotency key to return 200, got %d", resp2.Code)
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

func mustJSONAny(t *testing.T, body any) []byte {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	return b
}
