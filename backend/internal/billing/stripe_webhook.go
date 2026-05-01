package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// VerifyWebhookSignature checks Stripe-Signature header (t=, v1=).
func VerifyWebhookSignature(payload []byte, sigHeader string) error {
	secret := stripeWebhookSecret()
	if secret == "" {
		return fmt.Errorf("STRIPE_WEBHOOK_SECRET not set")
	}
	// t=timestamp,v1=signature
	var ts string
	var v1 string
	parts := strings.Split(sigHeader, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "t=") {
			ts = strings.TrimPrefix(p, "t=")
		}
		if strings.HasPrefix(p, "v1=") {
			v1 = strings.TrimPrefix(p, "v1=")
		}
	}
	if ts == "" || v1 == "" {
		return fmt.Errorf("invalid signature header")
	}
	ti, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	// Reject very old webhooks (5 min)
	if time.Since(time.Unix(ti, 0)) > 5*time.Minute || time.Until(time.Unix(ti, 0)) > 1*time.Minute {
		return fmt.Errorf("timestamp out of tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmacEqual(expected, v1) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// ReadWebhookBody returns raw body (caller should use http.MaxBytesReader on request).
func ReadWebhookBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}
