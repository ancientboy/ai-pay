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
	signaturePayload := buildPaySignaturePayload(agentDid, "m1", "10", "idem-e2e-1", now.Format(time.RFC3339))
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
