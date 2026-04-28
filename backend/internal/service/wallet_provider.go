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
