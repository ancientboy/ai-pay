package service

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type WalletProvider interface {
	Name() string
	CreateAddress(agentDID string, currency string, chainID string, mode string) (providerAccountID string, address string, err error)
	HealthCheck() error
}

type MockWalletProvider struct{}

func (m *MockWalletProvider) Name() string { return "mock" }

func (m *MockWalletProvider) CreateAddress(agentDID string, currency string, chainID string, mode string) (string, string, error) {
	a := strings.TrimSpace(agentDID)
	c := NormalizeCurrency(currency)
	ch := strings.TrimSpace(chainID)
	mde := strings.TrimSpace(mode)
	if a == "" || c == "" || ch == "" || mde == "" {
		return "", "", fmt.Errorf("invalid wallet provider input")
	}
	id := fmt.Sprintf("mock_%s_%s_%s_%s", strings.ReplaceAll(a, ":", "_"), strings.ToLower(c), strings.ToLower(ch), strings.ToLower(mde))
	addr := fmt.Sprintf("0x%s", strings.ToUpper(fmt.Sprintf("%x", id))[:40])
	return id, addr, nil
}

func (m *MockWalletProvider) HealthCheck() error { return nil }

type BridgeWalletProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func NewBridgeWalletProviderFromEnv() *BridgeWalletProvider {
	base := strings.TrimSpace(os.Getenv("BRIDGE_API_BASE_URL"))
	if base == "" {
		base = "https://api.bridge.xyz/v0"
	}
	return &BridgeWalletProvider{BaseURL: base, APIKey: strings.TrimSpace(os.Getenv("BRIDGE_API_KEY")), Client: &http.Client{Timeout: 15 * time.Second}}
}

func (b *BridgeWalletProvider) Name() string { return "bridge" }

func (b *BridgeWalletProvider) HealthCheck() error {
	if strings.TrimSpace(b.APIKey) == "" {
		return fmt.Errorf("bridge api key missing")
	}
	return nil
}

func (b *BridgeWalletProvider) CreateAddress(agentDID string, currency string, chainID string, mode string) (string, string, error) {
	if err := b.HealthCheck(); err != nil {
		return "", "", err
	}
	agent := strings.TrimSpace(agentDID)
	ccy := strings.ToLower(strings.TrimSpace(currency))
	chain := strings.ToLower(strings.TrimSpace(chainID))
	if agent == "" || ccy == "" || chain == "" {
		return "", "", fmt.Errorf("invalid bridge create address input")
	}
	customerID, err := b.ensureCustomer(agent)
	if err != nil {
		return "", "", err
	}
	vaID, address, err := b.createVirtualAccount(customerID, ccy, chain)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(address) == "" {
		address = vaID
	}
	return vaID, address, nil
}

func (b *BridgeWalletProvider) ensureCustomer(agentDID string) (string, error) {
	payload := map[string]any{
		"external_id": agentDID,
		"type":       "individual",
		"first_name": "Agent",
		"last_name":  "User",
		"email":      fmt.Sprintf("%s@example.local", strings.ReplaceAll(agentDID, ":", "_")),
	}
	resp, err := b.call("POST", "/customers", payload)
	if err != nil {
		return "", err
	}
	if id, ok := resp["id"].(string); ok && strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id), nil
	}
	return "", fmt.Errorf("bridge customer id missing")
}

func (b *BridgeWalletProvider) createVirtualAccount(customerID string, currency string, paymentRail string) (string, string, error) {
	dummyAddress := "0x0000000000000000000000000000000000000001"
	payload := map[string]any{
		"source": map[string]any{"currency": "usd"},
		"destination": map[string]any{
			"currency":     currency,
			"payment_rail": paymentRail,
			"address":      dummyAddress,
		},
	}
	path := fmt.Sprintf("/customers/%s/virtual_accounts", customerID)
	resp, err := b.call("POST", path, payload)
	if err != nil {
		return "", "", err
	}
	vaID, _ := resp["id"].(string)
	addr := ""
	if sdi, ok := resp["source_deposit_instructions"].(map[string]any); ok {
		if iban, ok := sdi["iban"].(string); ok {
			addr = iban
		}
		if acct, ok := sdi["bank_account_number"].(string); ok && strings.TrimSpace(addr) == "" {
			addr = acct
		}
	}
	return strings.TrimSpace(vaID), strings.TrimSpace(addr), nil
}

func (b *BridgeWalletProvider) VerifyWebhookSignature(payload []byte, signatureHeader string) error {
	parts := strings.Split(strings.TrimSpace(signatureHeader), ",")
	var tsRaw, sigRaw string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "t=") {
			tsRaw = strings.TrimPrefix(p, "t=")
		}
		if strings.HasPrefix(p, "v0=") {
			sigRaw = strings.TrimPrefix(p, "v0=")
		}
	}
	if tsRaw == "" || sigRaw == "" {
		return fmt.Errorf("invalid signature header")
	}
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	if now-ts > 10*60*1000 {
		return fmt.Errorf("bridge webhook timestamp too old")
	}
	pubKeyPEM := strings.TrimSpace(os.Getenv("BRIDGE_WEBHOOK_PUBLIC_KEY"))
	if pubKeyPEM == "" {
		return fmt.Errorf("bridge webhook public key missing")
	}
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return fmt.Errorf("invalid bridge webhook public key pem")
	}
	pk, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	rsaPK, ok := pk.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("bridge webhook key must be rsa")
	}
	signedPayload := tsRaw + "." + string(payload)
	h := sha256.Sum256([]byte(signedPayload))
	h = sha256.Sum256(h[:])
	sigBytes, err := base64.StdEncoding.DecodeString(sigRaw)
	if err != nil {
		return err
	}
	if err := rsa.VerifyPKCS1v15(rsaPK, crypto.SHA256, h[:], sigBytes); err != nil {
		return err
	}
	return nil
}

func (b *BridgeWalletProvider) call(method, path string, body any) (map[string]any, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(method, strings.TrimRight(b.BaseURL, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", b.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("bridge-%d", time.Now().UnixNano()))
	resp, err := b.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bridge api error status=%d body=%s", resp.StatusCode, string(data))
	}
	out := map[string]any{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	return out, nil
}

type StripeWalletProvider struct{}

func (s *StripeWalletProvider) Name() string { return "stripe" }

func (s *StripeWalletProvider) CreateAddress(agentDID string, currency string, chainID string, mode string) (string, string, error) {
	return "", "", fmt.Errorf("stripe wallet provider not implemented yet")
}

func (s *StripeWalletProvider) HealthCheck() error {
	if strings.TrimSpace(os.Getenv("STRIPE_API_KEY")) == "" {
		return fmt.Errorf("stripe api key missing")
	}
	return nil
}

type ProviderRouter interface {
	ResolveProvider(provider string) WalletProvider
	HealthCheck(provider string) error
}

type DefaultProviderRouter struct {
	providers map[string]WalletProvider
}

func NewDefaultProviderRouter() *DefaultProviderRouter {
	m := map[string]WalletProvider{}
	mock := &MockWalletProvider{}
	bridge := NewBridgeWalletProviderFromEnv()
	stripe := &StripeWalletProvider{}
	m[mock.Name()] = mock
	m[bridge.Name()] = bridge
	m[stripe.Name()] = stripe
	return &DefaultProviderRouter{providers: m}
}

func (r *DefaultProviderRouter) ResolveProvider(provider string) WalletProvider {
	if r == nil {
		return &MockWalletProvider{}
	}
	key := NormalizeWalletProvider(provider)
	if p, ok := r.providers[key]; ok {
		return p
	}
	if p, ok := r.providers["mock"]; ok {
		return p
	}
	return &MockWalletProvider{}
}

func (r *DefaultProviderRouter) HealthCheck(provider string) error {
	p := r.ResolveProvider(provider)
	if p == nil {
		return fmt.Errorf("provider unavailable")
	}
	return p.HealthCheck()
}

func NormalizeWalletProvider(raw string) string {
	p := strings.ToLower(strings.TrimSpace(raw))
	switch p {
	case "", "mock":
		return "mock"
	case "bridge", "stripe":
		return p
	default:
		return ""
	}
}
