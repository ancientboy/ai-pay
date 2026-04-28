package service

import (
	"fmt"
	"strings"
)

type WalletProvider interface {
	Name() string
	CreateAddress(agentDID string, currency string, chainID string, mode string) (providerAccountID string, address string, err error)
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


type ProviderRouter interface {
	ResolveProvider(provider string) WalletProvider
}

type DefaultProviderRouter struct {
	providers map[string]WalletProvider
}

func NewDefaultProviderRouter() *DefaultProviderRouter {
	m := map[string]WalletProvider{}
	mock := &MockWalletProvider{}
	m[mock.Name()] = mock
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
