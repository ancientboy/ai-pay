package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// SettlingWebhookPayload is sent to EXTERNAL_SETTLEMENT_WEBHOOK_URL when a pay order
// enters SETTLING (async channel). A relay can complete stablecoin / x402 settlement
// and then call POST /payment/status/callback on this backend.
type SettlingWebhookPayload struct {
	Event            string `json:"event"`
	TransactionID    string `json:"transactionId"`
	AgentDid         string `json:"agentDid"`
	MerchantID       string `json:"merchantId"`
	Amount           string `json:"amount"`
	IdempotencyKey   string `json:"idempotencyKey"`
	SignTimestampRFC string `json:"signTimestamp,omitempty"`
	CallbackHint     struct {
		Path   string `json:"path"`
		Method string `json:"method"`
		NoteZH string `json:"noteZh"`
		NoteEN string `json:"noteEn"`
	} `json:"callbackHint"`
}

func envSettlementWebhookURL() string {
	return strings.TrimSpace(os.Getenv("EXTERNAL_SETTLEMENT_WEBHOOK_URL"))
}

func envSettlementWebhookSecret() string {
	return strings.TrimSpace(os.Getenv("EXTERNAL_SETTLEMENT_WEBHOOK_SECRET"))
}

func envSettlementWebhookTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("EXTERNAL_SETTLEMENT_WEBHOOK_TIMEOUT_MS"))
	if raw == "" {
		return 8 * time.Second
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 100 || ms > 120_000 {
		return 8 * time.Second
	}
	return time.Duration(ms) * time.Millisecond
}

// notifyExternalSettling fires asynchronously; failures are logged only (payment already accepted).
func notifyExternalSettling(p SettlingWebhookPayload) {
	url := envSettlementWebhookURL()
	if url == "" {
		return
	}
	p.Event = "payment.settling"
	p.CallbackHint.Path = "/payment/status/callback"
	p.CallbackHint.Method = "POST"
	p.CallbackHint.NoteZH = "外部中继完成链上或 x402 后，向本服务回调 SETTLED / FAILED（需配置 CALLBACK_TOKEN 等，见 README）。"
	p.CallbackHint.NoteEN = "After on-chain or x402 settlement, POST SETTLED or FAILED to this service (see CALLBACK_TOKEN in README)."

	body, err := json.Marshal(p)
	if err != nil {
		log.Printf("settlement webhook: marshal: %v", err)
		return
	}
	secret := envSettlementWebhookSecret()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), envSettlementWebhookTimeout())
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			log.Printf("settlement webhook: new request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "AgentTrustPay-SettlementHook/1.0")
		if secret != "" {
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write(body)
			sum := mac.Sum(nil)
			req.Header.Set("X-AgentTrust-Signature", "sha256="+hex.EncodeToString(sum))
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("settlement webhook: post %s: %v", url, err)
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("settlement webhook: unexpected status %d from %s", resp.StatusCode, url)
		}
	}()
}
