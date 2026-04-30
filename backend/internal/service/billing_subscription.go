package service

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// BillingSubscriptionView is the current subscription state for a console user.
type BillingSubscriptionView struct {
	UserID                 string `json:"userId"`
	PlanCode               string `json:"planCode"`
	Status                 string `json:"status"`
	Currency               string `json:"currency"`
	Provider               string `json:"provider"`
	ProviderCustomerID     string `json:"providerCustomerId,omitempty"`
	ProviderSubscriptionID string `json:"providerSubscriptionId,omitempty"`
	CurrentPeriodEnd       string `json:"currentPeriodEnd,omitempty"`
	CancelAtPeriodEnd      bool   `json:"cancelAtPeriodEnd"`
}

// BillingCheckoutSessionInput records a created checkout session (for support / idempotency).
type BillingCheckoutSessionInput struct {
	LocalID                string
	UserID                 string
	Provider               string
	PlanCode               string
	PaymentRail            string
	Currency               string
	AmountMinor            *int64
	Status                 string
	CheckoutURL            string
	ProviderSessionID      string
	ProviderSubscriptionID string
	Metadata               map[string]string
}

type BillingCheckoutSessionUpdate struct {
	ProviderSessionID string
	Status            string
}

// BillingSubscriptionUpsert updates subscription row after Stripe webhook or sync.
type BillingSubscriptionUpsert struct {
	UserID                 string
	PlanCode               string
	Status                 string
	Currency               string
	Provider               string
	ProviderCustomerID     string
	ProviderSubscriptionID string
	CurrentPeriodEnd       *time.Time
	CancelAtPeriodEnd      bool
	RawEvent               json.RawMessage
}

// --- in-memory implementation ---

type billingMem struct {
	mu           sync.RWMutex
	byUser       map[string]BillingSubscriptionView
	checkoutByID map[string]BillingCheckoutSessionInput
}

// initBillingMem lazily allocates billing state on in-memory service.
func (s *Service) initBillingMem() {
	if s.billing != nil {
		return
	}
	s.billing = &billingMem{
		byUser:       map[string]BillingSubscriptionView{},
		checkoutByID: map[string]BillingCheckoutSessionInput{},
	}
}

func (s *Service) RecordBillingCheckoutSession(in BillingCheckoutSessionInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initBillingMem()
	s.billing.checkoutByID[in.LocalID] = in
	return nil
}

func (s *Service) UpdateBillingCheckoutSessionByProviderSession(in BillingCheckoutSessionUpdate) error {
	if strings.TrimSpace(in.ProviderSessionID) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.billing == nil {
		return nil
	}
	for id, v := range s.billing.checkoutByID {
		if strings.TrimSpace(v.ProviderSessionID) == strings.TrimSpace(in.ProviderSessionID) {
			v.Status = in.Status
			s.billing.checkoutByID[id] = v
			break
		}
	}
	return nil
}

func (s *Service) GetBillingSubscription(userID string) (BillingSubscriptionView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.billing == nil {
		return BillingSubscriptionView{}, false
	}
	v, ok := s.billing.byUser[userID]
	return v, ok
}

func (s *Service) UpsertBillingSubscription(u BillingSubscriptionUpsert) error {
	if u.UserID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initBillingMem()
	cpe := ""
	if u.CurrentPeriodEnd != nil && !u.CurrentPeriodEnd.IsZero() {
		cpe = u.CurrentPeriodEnd.UTC().Format(time.RFC3339)
	}
	s.billing.byUser[u.UserID] = BillingSubscriptionView{
		UserID:                 u.UserID,
		PlanCode:               u.PlanCode,
		Status:                 u.Status,
		Currency:               u.Currency,
		Provider:               u.Provider,
		ProviderCustomerID:     u.ProviderCustomerID,
		ProviderSubscriptionID: u.ProviderSubscriptionID,
		CurrentPeriodEnd:       cpe,
		CancelAtPeriodEnd:      u.CancelAtPeriodEnd,
	}
	return nil
}
