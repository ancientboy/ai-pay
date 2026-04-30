package service

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func (s *PersistentService) RecordBillingCheckoutSession(in BillingCheckoutSessionInput) error {
	var metaStr interface{}
	if in.Metadata != nil {
		b, _ := json.Marshal(in.Metadata)
		metaStr = string(b)
	} else {
		metaStr = nil
	}
	var amount interface{}
	if in.AmountMinor != nil {
		amount = *in.AmountMinor
	} else {
		amount = nil
	}
	_, err := s.store.DB.Exec(`
INSERT INTO billing_checkout_session
  (id, user_id, provider, plan_code, payment_rail, currency, amount_minor, status, checkout_url,
   provider_session_id, provider_subscription_id, metadata_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(), UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE
  status=VALUES(status), checkout_url=VALUES(checkout_url), provider_session_id=VALUES(provider_session_id),
  provider_subscription_id=VALUES(provider_subscription_id), metadata_json=VALUES(metadata_json), updated_at=UTC_TIMESTAMP()`,
		in.LocalID, in.UserID, in.Provider, in.PlanCode, in.PaymentRail, in.Currency, amount, in.Status, in.CheckoutURL,
		nullIfEmpty(in.ProviderSessionID), nullIfEmpty(in.ProviderSubscriptionID), metaStr,
	)
	return err
}

func (s *PersistentService) UpdateBillingCheckoutSessionByProviderSession(in BillingCheckoutSessionUpdate) error {
	if strings.TrimSpace(in.ProviderSessionID) == "" {
		return nil
	}
	_, err := s.store.DB.Exec(`
UPDATE billing_checkout_session
   SET status = ?, updated_at = UTC_TIMESTAMP()
 WHERE provider_session_id = ?`,
		strings.TrimSpace(in.Status),
		strings.TrimSpace(in.ProviderSessionID),
	)
	return err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *PersistentService) GetBillingSubscription(userID string) (BillingSubscriptionView, bool) {
	if userID == "" {
		return BillingSubscriptionView{}, false
	}
	var v BillingSubscriptionView
	var provSub, provCust sql.NullString
	var periodEnd sql.NullTime
	var cancelAt int
	var raw sql.NullString
	err := s.store.DB.QueryRow(`
SELECT user_id, plan_code, status, currency, provider, provider_customer_id, provider_subscription_id,
       current_period_end, cancel_at_period_end, raw_event_json
  FROM billing_subscription WHERE user_id = ?`, userID).Scan(
		&v.UserID, &v.PlanCode, &v.Status, &v.Currency, &v.Provider, &provCust, &provSub, &periodEnd, &cancelAt, &raw,
	)
	if err == sql.ErrNoRows {
		return BillingSubscriptionView{}, false
	}
	if err != nil {
		return BillingSubscriptionView{}, false
	}
	if provSub.Valid {
		v.ProviderSubscriptionID = provSub.String
	}
	if provCust.Valid {
		v.ProviderCustomerID = provCust.String
	}
	if periodEnd.Valid {
		v.CurrentPeriodEnd = periodEnd.Time.UTC().Format(time.RFC3339)
	}
	v.CancelAtPeriodEnd = cancelAt != 0
	_ = raw
	return v, true
}

func (s *PersistentService) UpsertBillingSubscription(u BillingSubscriptionUpsert) error {
	if u.UserID == "" {
		return nil
	}
	var period interface{}
	if u.CurrentPeriodEnd != nil && !u.CurrentPeriodEnd.IsZero() {
		period = u.CurrentPeriodEnd.UTC()
	} else {
		period = nil
	}
	cancel := 0
	if u.CancelAtPeriodEnd {
		cancel = 1
	}
	var raw interface{}
	if len(u.RawEvent) > 0 {
		raw = string(u.RawEvent)
	} else {
		raw = nil
	}
	_, err := s.store.DB.Exec(`
INSERT INTO billing_subscription
  (user_id, plan_code, status, currency, provider, provider_customer_id, provider_subscription_id,
   current_period_end, cancel_at_period_end, raw_event_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(), UTC_TIMESTAMP())
ON DUPLICATE KEY UPDATE
  plan_code=VALUES(plan_code), status=VALUES(status), currency=VALUES(currency), provider=VALUES(provider),
  provider_customer_id=VALUES(provider_customer_id), provider_subscription_id=VALUES(provider_subscription_id),
  current_period_end=VALUES(current_period_end), cancel_at_period_end=VALUES(cancel_at_period_end),
  raw_event_json=VALUES(raw_event_json), updated_at=UTC_TIMESTAMP()`,
		u.UserID, u.PlanCode, u.Status, u.Currency, u.Provider,
		nullIfEmpty(u.ProviderCustomerID), nullIfEmpty(u.ProviderSubscriptionID), period, cancel, raw,
	)
	return err
}
