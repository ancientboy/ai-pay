package http

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ai-pay-backend/internal/service"
)

func TestMVP8EndpointsFlow(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	svc := service.New()
	server := NewServerForTest(svc, func() time.Time { return now }, 1000, 1000)
	handler := server.Routes()

	agentDid := "did:gusd:agent:e2e_1"
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pubBase64 := base64.StdEncoding.EncodeToString(pub)

	// 1) register
	registerBody := mustJSON(t, map[string]any{"agentDid": agentDid, "didPubKey": pubBase64})
	authHeaders := map[string]string{"X-User-Id": "e2e-owner"}
	registerResp := performRequest(t, handler, http.MethodPost, "/agent/did/register", registerBody, authHeaders)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status=%d", registerResp.Code)
	}

	// 2) create account
	createBody := mustJSON(t, map[string]any{"agentDid": agentDid})
	createResp := performRequest(t, handler, http.MethodPost, "/account/create", createBody, authHeaders)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create account status=%d", createResp.Code)
	}
	createData := decodeData(t, createResp)
	vaID, _ := createData["VAAccountID"].(string)
	if vaID == "" {
		t.Fatalf("missing VAAccountID in response")
	}

	// 3) recharge
	rechargeBody := mustJSON(t, map[string]any{
		"vaAccountId": vaID,
		"amount":      "100",
	})
	rechargeAllHeaders := map[string]string{"Idempotency-Key": "rch-e2e-1", "X-User-Id": "e2e-owner"}
	rechargeResp := performRequest(t, handler, http.MethodPost, "/fund/recharge", rechargeBody, rechargeAllHeaders)
	if rechargeResp.Code != http.StatusOK {
		t.Fatalf("recharge status=%d", rechargeResp.Code)
	}

	// 4) authorize
	authBody := mustJSON(t, map[string]any{
		"agentDid":    agentDid,
		"singleLimit": "50",
		"dailyLimit":  "200",
		"whitelist":   []string{"m1"},
	})
	authResp := performRequest(t, handler, http.MethodPost, "/authorize/payment/set", authBody, authHeaders)
	if authResp.Code != http.StatusOK {
		t.Fatalf("authorize status=%d", authResp.Code)
	}

	// 5) pay
	signaturePayload := buildPaySignaturePayload(agentDid, "m1", "GUSD", "10", "idem-e2e-1", now.Format(time.RFC3339))
	payBody := mustJSON(t, map[string]any{
		"payerDid":   agentDid,
		"merchantId": "m1",
		"amount":     "10",
		"signature":  base64.StdEncoding.EncodeToString(ed25519.Sign(priv, signaturePayload)),
	})
	payHeaders := map[string]string{
		"Idempotency-Key":  "idem-e2e-1",
		"X-Sign-Timestamp": now.Format(time.RFC3339),
		"X-User-Id":        "e2e-owner",
	}
	payResp := performRequest(t, handler, http.MethodPost, "/payment/x402/pay", payBody, payHeaders)
	if payResp.Code != http.StatusOK {
		t.Fatalf("pay status=%d body=%s", payResp.Code, payResp.Body.String())
	}
	payData := decodeData(t, payResp)
	txID, _ := payData["transactionId"].(string)
	if txID == "" {
		t.Fatalf("missing transactionId in pay response")
	}

	// 6) status
	statusResp := performRequest(t, handler, http.MethodGet, "/payment/status/query?transactionId="+txID, nil, authHeaders)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status query status=%d", statusResp.Code)
	}

	// 7) balance
	balanceResp := performRequest(t, handler, http.MethodGet, "/account/balance/query?accountId="+vaID, nil, authHeaders)
	if balanceResp.Code != http.StatusOK {
		t.Fatalf("balance query status=%d", balanceResp.Code)
	}
	balanceData := decodeData(t, balanceResp)
	balance, ok := balanceData["balance"].(float64)
	if !ok {
		t.Fatalf("missing balance field")
	}
	if balance != 90 {
		t.Fatalf("expected balance 90 got %v", balance)
	}

	// 8) ledger
	ledgerResp := performRequest(t, handler, http.MethodGet, "/account/ledger/query?accountId="+vaID, nil, authHeaders)
	if ledgerResp.Code != http.StatusOK {
		t.Fatalf("ledger query status=%d", ledgerResp.Code)
	}
}

func TestOrchestrationIntentFlow_Success(t *testing.T) {
	now := time.Date(2026, 4, 29, 3, 0, 0, 0, time.UTC)
	svc := service.New()
	server := NewServerForTest(svc, func() time.Time { return now }, 1000, 1000)
	handler := server.Routes()
	headers := map[string]string{"X-User-Id": "orch-owner"}

	agentDid := "did:orch:agent:success"
	vaID := registerAndCreateAccount(t, handler, headers, agentDid)

	bindBody := mustJSON(t, map[string]any{
		"platformVaAccountId": vaID,
		"provider":            "bridge",
		"providerCustomerId":  "cus_001",
		"providerAccountId":   "acc_001",
		"currency":            "USDC",
		"metadata":            `{"tier":"prod"}`,
	})
	bindResp := performRequest(t, handler, http.MethodPost, "/orchestrate/provider-account/bind", bindBody, headers)
	if bindResp.Code != http.StatusOK {
		t.Fatalf("bind provider account status=%d body=%s", bindResp.Code, bindResp.Body.String())
	}

	createBody := mustJSON(t, map[string]any{
		"platformVaAccountId": vaID,
		"agentDid":            agentDid,
		"merchantId":          "m_orch_1",
		"currency":            "USDC",
		"amount":              "12.5",
	})
	createResp := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/create", createBody, headers)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create intent status=%d body=%s", createResp.Code, createResp.Body.String())
	}
	intent := decodeData(t, createResp)
	intentID, _ := intent["intentId"].(string)
	if intentID == "" {
		t.Fatalf("missing intentId")
	}
	if gotStatus, _ := intent["status"].(string); gotStatus != "CREATED" {
		t.Fatalf("unexpected initial status: %v", gotStatus)
	}

	executeBody := mustJSON(t, map[string]any{"intentId": intentID, "provider": "bridge"})
	executeResp := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/execute", executeBody, headers)
	if executeResp.Code != http.StatusOK {
		t.Fatalf("execute intent status=%d body=%s", executeResp.Code, executeResp.Body.String())
	}
	execData := decodeData(t, executeResp)
	if gotProvider, _ := execData["provider"].(string); gotProvider != "bridge" {
		t.Fatalf("unexpected provider: %v", gotProvider)
	}
	if gotStatus, _ := execData["status"].(string); gotStatus != "SUCCESS" {
		t.Fatalf("unexpected execution status: %v", gotStatus)
	}

	statusResp := performRequest(t, handler, http.MethodGet, "/orchestrate/payment-intent/status?intentId="+intentID, nil, headers)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status intent status=%d body=%s", statusResp.Code, statusResp.Body.String())
	}
	statusPayload := decodeData(t, statusResp)
	intentObj, ok := statusPayload["intent"].(map[string]any)
	if !ok {
		t.Fatalf("status response missing intent object")
	}
	if got, _ := intentObj["status"].(string); got != "SETTLED" {
		t.Fatalf("expected intent status SETTLED got=%v", got)
	}
	execs, ok := statusPayload["executions"].([]any)
	if !ok || len(execs) != 1 {
		t.Fatalf("expected 1 execution record got=%v", statusPayload["executions"])
	}
}

func TestOrchestrationIntentFlow_ExecuteFailsWhenNoRoute(t *testing.T) {
	now := time.Date(2026, 4, 29, 3, 10, 0, 0, time.UTC)
	svc := service.New()
	server := NewServerForTest(svc, func() time.Time { return now }, 1000, 1000)
	handler := server.Routes()
	headers := map[string]string{"X-User-Id": "orch-owner-no-route"}

	agentDid := "did:orch:agent:no-route"
	vaID := registerAndCreateAccount(t, handler, headers, agentDid)

	createBody := mustJSON(t, map[string]any{
		"platformVaAccountId": vaID,
		"agentDid":            agentDid,
		"merchantId":          "m_orch_2",
		"currency":            "USDT",
		"amount":              "8",
	})
	createResp := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/create", createBody, headers)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create intent status=%d body=%s", createResp.Code, createResp.Body.String())
	}
	intent := decodeData(t, createResp)
	intentID, _ := intent["intentId"].(string)
	if intentID == "" {
		t.Fatalf("missing intentId")
	}

	executeBody := mustJSON(t, map[string]any{"intentId": intentID, "provider": "bridge"})
	executeResp := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/execute", executeBody, headers)
	if executeResp.Code != http.StatusBadRequest {
		t.Fatalf("expect bad request for no route; got=%d body=%s", executeResp.Code, executeResp.Body.String())
	}
	code, msg := decodeError(t, executeResp)
	if code != "PAY-010" || msg == "" {
		t.Fatalf("unexpected error payload code=%s msg=%s", code, msg)
	}
}

func TestOrchestrationIntentFlow_ExecuteFailsWhenReplayed(t *testing.T) {
	now := time.Date(2026, 4, 29, 3, 20, 0, 0, time.UTC)
	svc := service.New()
	server := NewServerForTest(svc, func() time.Time { return now }, 1000, 1000)
	handler := server.Routes()
	headers := map[string]string{"X-User-Id": "orch-owner-replay"}

	agentDid := "did:orch:agent:replay"
	vaID := registerAndCreateAccount(t, handler, headers, agentDid)
	bindProvider(t, handler, headers, vaID, "bridge", "USDC", "acc_replay")
	intentID := createIntent(t, handler, headers, vaID, agentDid, "USDC", "3")

	firstExec := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/execute", mustJSON(t, map[string]any{"intentId": intentID, "provider": "bridge"}), headers)
	if firstExec.Code != http.StatusOK {
		t.Fatalf("first execute failed status=%d body=%s", firstExec.Code, firstExec.Body.String())
	}
	secondExec := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/execute", mustJSON(t, map[string]any{"intentId": intentID, "provider": "bridge"}), headers)
	if secondExec.Code != http.StatusBadRequest {
		t.Fatalf("second execute should fail; status=%d body=%s", secondExec.Code, secondExec.Body.String())
	}
	code, msg := decodeError(t, secondExec)
	if code != "PAY-010" || msg == "" {
		t.Fatalf("unexpected replay error payload code=%s msg=%s", code, msg)
	}
}

func TestOrchestrationIntentFlow_ExecuteFailsWhenProviderMismatch(t *testing.T) {
	now := time.Date(2026, 4, 29, 3, 30, 0, 0, time.UTC)
	svc := service.New()
	server := NewServerForTest(svc, func() time.Time { return now }, 1000, 1000)
	handler := server.Routes()
	headers := map[string]string{"X-User-Id": "orch-owner-mismatch"}

	agentDid := "did:orch:agent:mismatch"
	vaID := registerAndCreateAccount(t, handler, headers, agentDid)
	bindProvider(t, handler, headers, vaID, "bridge", "USDC", "acc_mismatch")
	intentID := createIntent(t, handler, headers, vaID, agentDid, "USDC", "6")

	execResp := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/execute", mustJSON(t, map[string]any{"intentId": intentID, "provider": "stripe"}), headers)
	if execResp.Code != http.StatusBadRequest {
		t.Fatalf("expect provider mismatch fail; status=%d body=%s", execResp.Code, execResp.Body.String())
	}
	code, msg := decodeError(t, execResp)
	if code != "PAY-010" || msg == "" {
		t.Fatalf("unexpected mismatch error payload code=%s msg=%s", code, msg)
	}
}

func performRequest(t *testing.T, handler http.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader([]byte{})
	} else {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json err: %v", err)
	}
	return b
}

func decodeData(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response err: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("response missing data object")
	}
	return data
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response err: %v", err)
	}
	code, _ := payload["code"].(string)
	msg, _ := payload["message"].(string)
	return code, msg
}

func registerAndCreateAccount(t *testing.T, handler http.Handler, headers map[string]string, agentDid string) string {
	t.Helper()
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	registerBody := mustJSON(t, map[string]any{
		"agentDid":  agentDid,
		"didPubKey": base64.StdEncoding.EncodeToString(pub),
	})
	registerResp := performRequest(t, handler, http.MethodPost, "/agent/did/register", registerBody, headers)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status=%d body=%s", registerResp.Code, registerResp.Body.String())
	}
	createBody := mustJSON(t, map[string]any{"agentDid": agentDid})
	createResp := performRequest(t, handler, http.MethodPost, "/account/create", createBody, headers)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create account status=%d body=%s", createResp.Code, createResp.Body.String())
	}
	data := decodeData(t, createResp)
	vaID, _ := data["VAAccountID"].(string)
	if vaID == "" {
		t.Fatalf("missing VAAccountID")
	}
	return vaID
}

func bindProvider(t *testing.T, handler http.Handler, headers map[string]string, vaID string, provider string, currency string, providerAccountID string) {
	t.Helper()
	body := mustJSON(t, map[string]any{
		"platformVaAccountId": vaID,
		"provider":            provider,
		"providerCustomerId":  "cust_" + providerAccountID,
		"providerAccountId":   providerAccountID,
		"currency":            currency,
		"metadata":            fmt.Sprintf(`{"account":"%s"}`, providerAccountID),
	})
	resp := performRequest(t, handler, http.MethodPost, "/orchestrate/provider-account/bind", body, headers)
	if resp.Code != http.StatusOK {
		t.Fatalf("bind provider status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func createIntent(t *testing.T, handler http.Handler, headers map[string]string, vaID string, agentDid string, currency string, amount string) string {
	t.Helper()
	body := mustJSON(t, map[string]any{
		"platformVaAccountId": vaID,
		"agentDid":            agentDid,
		"merchantId":          "m_" + agentDid,
		"currency":            currency,
		"amount":              amount,
	})
	resp := performRequest(t, handler, http.MethodPost, "/orchestrate/payment-intent/create", body, headers)
	if resp.Code != http.StatusOK {
		t.Fatalf("create intent status=%d body=%s", resp.Code, resp.Body.String())
	}
	data := decodeData(t, resp)
	intentID, _ := data["intentId"].(string)
	if intentID == "" {
		t.Fatalf("missing intentId")
	}
	return intentID
}
