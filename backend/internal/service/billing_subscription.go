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

type BillingCheckoutSessionView struct {
	LocalID           string            `json:"localId"`
	UserID            string            `json:"userId"`
	Provider          string            `json:"provider"`
	PlanCode          string            `json:"planCode"`
	PaymentRail       string            `json:"paymentRail"`
	Currency          string            `json:"currency"`
	AmountMinor       int64             `json:"amountMinor"`
	Status            string            `json:"status"`
	ProviderSessionID string            `json:"providerSessionId,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type BillingReconciliationEntry struct {
	ProviderSessionID string `json:"providerSessionId"`
	UserID            string `json:"userId"`
	VAAccountID       string `json:"vaAccountId,omitempty"`
	CheckoutType      string `json:"checkoutType,omitempty"`
	Currency          string `json:"currency"`
	AmountMinor       int64  `json:"amountMinor"`
	CheckoutStatus    string `json:"checkoutStatus"`
	CreatedAt         string `json:"createdAt"`
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

func (s *Service) ListBillingCheckoutSessions(userID string, limit int, offset int) []BillingCheckoutSessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.billing == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	items := make([]BillingCheckoutSessionView, 0, len(s.billing.checkoutByID))
	for _, v := range s.billing.checkoutByID {
		if strings.TrimSpace(v.UserID) != strings.TrimSpace(userID) {
			continue
		}
		amt := int64(0)
		if v.AmountMinor != nil {
			amt = *v.AmountMinor
		}
		items = append(items, BillingCheckoutSessionView{
			LocalID:           v.LocalID,
			UserID:            v.UserID,
			Provider:          v.Provider,
			PlanCode:          v.PlanCode,
			PaymentRail:       v.PaymentRail,
			Currency:          v.Currency,
			AmountMinor:       amt,
			Status:            v.Status,
			ProviderSessionID: v.ProviderSessionID,
			Metadata:          v.Metadata,
		})
	}
	if offset >= len(items) {
		return []BillingCheckoutSessionView{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func (s *Service) ListBillingReconciliation(userID string, limit int, offset int) []BillingReconciliationEntry {
	sessions := s.ListBillingCheckoutSessions(userID, limit, offset)
	out := make([]BillingReconciliationEntry, 0, len(sessions))
	for _, item := range sessions {
		entry := BillingReconciliationEntry{
			ProviderSessionID: item.ProviderSessionID,
			UserID:            item.UserID,
			Currency:          item.Currency,
			AmountMinor:       item.AmountMinor,
			CheckoutStatus:    item.Status,
			CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		}
		if item.Metadata != nil {
			entry.VAAccountID = strings.TrimSpace(item.Metadata["va_account_id"])
			entry.CheckoutType = strings.TrimSpace(item.Metadata["checkout_type"])
		}
		if entry.CheckoutType == "" {
			entry.CheckoutType = "subscription"
		}
		out = append(out, entry)
	}
	return out
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
