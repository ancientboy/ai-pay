package service

import (
	"fmt"
	"strings"
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
	cobo := &CoboWalletProvider{}
	m[mock.Name()] = mock
	m[cobo.Name()] = cobo
	return &DefaultProviderRouter{providers: m}
}

func (r *DefaultProviderRouter) ResolveProvider(provider string) WalletProvider {
	if r == nil {
		return &MockWalletProvider{}
	}
	key := strings.ToLower(strings.TrimSpace(provider))
	if p, ok := r.providers[key]; ok {
		return p
	}
	if p, ok := r.providers["mock"]; ok {
		return p
	}
	return &MockWalletProvider{}
}


type CoboWalletProvider struct{}

func (c *CoboWalletProvider) Name() string { return "cobo" }

func (c *CoboWalletProvider) CreateAddress(agentDID string, currency string, chainID string, mode string) (string, string, error) {
	a := strings.TrimSpace(agentDID)
	ccy := NormalizeCurrency(currency)
	ch := strings.TrimSpace(chainID)
	m := strings.TrimSpace(mode)
	if a == "" || ccy == "" || ch == "" || m == "" {
		return "", "", fmt.Errorf("invalid cobo provider input")
	}
	// Skeleton only: real Cobo API call will be injected in next step.
	providerID := fmt.Sprintf("cobo_stub_%s_%s_%s_%s", strings.ReplaceAll(a, ":", "_"), strings.ToLower(ccy), strings.ToLower(ch), strings.ToLower(m))
	address := fmt.Sprintf("0x%s", strings.ToLower(fmt.Sprintf("%040x", len(providerID)+len(a)+len(ccy)+len(ch)+len(m))))
	return providerID, address, nil
}

func (c *CoboWalletProvider) HealthCheck() error {
	// Skeleton only: should verify API key / endpoint reachability.
	return nil
}

func NormalizeWalletProvider(raw string) string {
	p := strings.ToLower(strings.TrimSpace(raw))
	switch p {
	case "", "mock":
		return "mock"
	case "cobo":
		return "cobo"
	default:
		return ""
	}
}

func (r *DefaultProviderRouter) HealthCheck(provider string) error {
	p := r.ResolveProvider(provider)
	if p == nil {
		return fmt.Errorf("provider unavailable")
	}
	return p.HealthCheck()
}
