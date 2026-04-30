package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const stripeAPI = "https://api.stripe.com/v1"

// CheckoutSessionResult is returned after creating a Stripe Checkout Session.
type CheckoutSessionResult struct {
	ID     string
	URL    string
	Status string
}

type PaymentIntentChargeSummary struct {
	ID             string
	AmountRefunded int64
	Currency       string
}

func stripeSecretKey() string {
	return strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY"))
}

func stripeWebhookSecret() string {
	return strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
}

// PriceIDForPlan maps plan codes to Stripe Price IDs (recurring).
func PriceIDForPlan(planCode string) (string, bool) {
	planCode = strings.ToLower(strings.TrimSpace(planCode))
	switch planCode {
	case "starter":
		v := strings.TrimSpace(os.Getenv("STRIPE_PRICE_STARTER"))
		return v, v != ""
	case "growth":
		v := strings.TrimSpace(os.Getenv("STRIPE_PRICE_GROWTH"))
		return v, v != ""
	default:
		return "", false
	}
}

// CreateSubscriptionCheckout calls Stripe to create a subscription Checkout Session.
func CreateSubscriptionCheckout(priceID, successURL, cancelURL string, metadata map[string]string) (CheckoutSessionResult, error) {
	sk := stripeSecretKey()
	if sk == "" {
		return CheckoutSessionResult{}, fmt.Errorf("STRIPE_SECRET_KEY not configured")
	}
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("line_items[0][price]", priceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("subscription_data[metadata][plan_code]", metadata["plan_code"])
	form.Set("subscription_data[metadata][user_id]", metadata["user_id"])
	form.Set("metadata[plan_code]", metadata["plan_code"])
	form.Set("metadata[user_id]", metadata["user_id"])

	req, err := http.NewRequest(http.MethodPost, stripeAPI+"/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return CheckoutSessionResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+sk)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CheckoutSessionResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CheckoutSessionResult{}, fmt.Errorf("stripe checkout failed: %s: %s", resp.Status, truncate(string(body), 500))
	}
	var parsed struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return CheckoutSessionResult{}, fmt.Errorf("stripe response decode: %w", err)
	}
	if parsed.URL == "" {
		return CheckoutSessionResult{}, fmt.Errorf("stripe returned empty checkout url")
	}
	return CheckoutSessionResult{ID: parsed.ID, URL: parsed.URL, Status: parsed.Status}, nil
}

// CreatePaymentCheckout creates a one-time Checkout Session for custom top-up amounts.
func CreatePaymentCheckout(amountMinor int64, currency, successURL, cancelURL string, metadata map[string]string) (CheckoutSessionResult, error) {
	sk := stripeSecretKey()
	if sk == "" {
		return CheckoutSessionResult{}, fmt.Errorf("STRIPE_SECRET_KEY not configured")
	}
	if amountMinor <= 0 {
		return CheckoutSessionResult{}, fmt.Errorf("invalid amount")
	}
	currency = strings.ToLower(strings.TrimSpace(currency))
	if currency == "" {
		currency = "usd"
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("success_url", successURL)
	form.Set("cancel_url", cancelURL)
	form.Set("line_items[0][price_data][currency]", currency)
	form.Set("line_items[0][price_data][product_data][name]", "AI Pay Card Top-up")
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(amountMinor, 10))
	form.Set("line_items[0][quantity]", "1")
	for k, v := range metadata {
		if strings.TrimSpace(k) == "" {
			continue
		}
		form.Set("metadata["+k+"]", v)
		form.Set("payment_intent_data[metadata]["+k+"]", v)
	}

	req, err := http.NewRequest(http.MethodPost, stripeAPI+"/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return CheckoutSessionResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+sk)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CheckoutSessionResult{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CheckoutSessionResult{}, fmt.Errorf("stripe checkout failed: %s: %s", resp.Status, truncate(string(body), 500))
	}
	var parsed struct {
		ID     string `json:"id"`
		URL    string `json:"url"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return CheckoutSessionResult{}, fmt.Errorf("stripe response decode: %w", err)
	}
	if parsed.URL == "" {
		return CheckoutSessionResult{}, fmt.Errorf("stripe returned empty checkout url")
	}
	return CheckoutSessionResult{ID: parsed.ID, URL: parsed.URL, Status: parsed.Status}, nil
}

// FindCheckoutSessionByPaymentIntent finds checkout session id and metadata from a payment_intent id.
func FindCheckoutSessionByPaymentIntent(paymentIntentID string) (sessionID string, metadata map[string]string, err error) {
	sk := stripeSecretKey()
	if sk == "" || strings.TrimSpace(paymentIntentID) == "" {
		return "", nil, fmt.Errorf("missing stripe config or payment_intent id")
	}
	req, err := http.NewRequest(http.MethodGet, stripeAPI+"/checkout/sessions?payment_intent="+url.QueryEscape(strings.TrimSpace(paymentIntentID))+"&limit=1", nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sk)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("stripe checkout list failed: %s: %s", resp.Status, truncate(string(body), 500))
	}
	var parsed struct {
		Data []struct {
			ID       string            `json:"id"`
			Metadata map[string]string `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", nil, err
	}
	if len(parsed.Data) == 0 {
		return "", nil, fmt.Errorf("checkout session not found for payment_intent")
	}
	return parsed.Data[0].ID, parsed.Data[0].Metadata, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// RetrieveCheckoutSession fetches a Checkout Session including subscription id.
func RetrieveCheckoutSession(sessionID string) (subscriptionID string, customerID string, meta map[string]string, err error) {
	sk := stripeSecretKey()
	if sk == "" || strings.TrimSpace(sessionID) == "" {
		return "", "", nil, fmt.Errorf("missing stripe config or session id")
	}
	req, err := http.NewRequest(http.MethodGet, stripeAPI+"/checkout/sessions/"+url.PathEscape(sessionID)+"?expand[]=subscription", nil)
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sk)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", nil, fmt.Errorf("stripe session retrieve failed: %s: %s", resp.Status, truncate(string(body), 500))
	}
	var parsed struct {
		Subscription json.RawMessage   `json:"subscription"`
		Customer     json.RawMessage   `json:"customer"`
		Metadata     map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", nil, err
	}
	sub := ""
	if len(parsed.Subscription) > 0 && parsed.Subscription[0] == '"' {
		_ = json.Unmarshal(parsed.Subscription, &sub)
	} else if len(parsed.Subscription) > 0 {
		var subObj struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(parsed.Subscription, &subObj) == nil {
			sub = subObj.ID
		}
	}
	cust := ""
	if len(parsed.Customer) > 0 && parsed.Customer[0] == '"' {
		_ = json.Unmarshal(parsed.Customer, &cust)
	} else if len(parsed.Customer) > 0 {
		var cObj struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(parsed.Customer, &cObj) == nil {
			cust = cObj.ID
		}
	}
	return sub, cust, parsed.Metadata, nil
}

func RetrievePaymentIntentCharges(paymentIntentID string) ([]PaymentIntentChargeSummary, error) {
	sk := stripeSecretKey()
	if sk == "" || strings.TrimSpace(paymentIntentID) == "" {
		return nil, fmt.Errorf("missing stripe config or payment_intent id")
	}
	req, err := http.NewRequest(http.MethodGet, stripeAPI+"/payment_intents/"+url.PathEscape(paymentIntentID)+"?expand[]=charges.data", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sk)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("stripe payment_intent retrieve failed: %s: %s", resp.Status, truncate(string(body), 500))
	}
	var parsed struct {
		Charges struct {
			Data []struct {
				ID             string `json:"id"`
				AmountRefunded int64  `json:"amount_refunded"`
				Currency       string `json:"currency"`
			} `json:"data"`
		} `json:"charges"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]PaymentIntentChargeSummary, 0, len(parsed.Charges.Data))
	for _, c := range parsed.Charges.Data {
		out = append(out, PaymentIntentChargeSummary{
			ID:             strings.TrimSpace(c.ID),
			AmountRefunded: c.AmountRefunded,
			Currency:       strings.ToUpper(strings.TrimSpace(c.Currency)),
		})
	}
	return out, nil
}

// RetrieveSubscription loads subscription status from Stripe (includes metadata and billing currency).
func RetrieveSubscription(subscriptionID string) (status string, currency string, cancelAtPeriodEnd bool, currentPeriodEnd time.Time, metadata map[string]string, err error) {
	sk := stripeSecretKey()
	if sk == "" || strings.TrimSpace(subscriptionID) == "" {
		return "", "", false, time.Time{}, nil, fmt.Errorf("missing stripe config or subscription id")
	}
	req, err := http.NewRequest(http.MethodGet, stripeAPI+"/subscriptions/"+url.PathEscape(subscriptionID), nil)
	if err != nil {
		return "", "", false, time.Time{}, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sk)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", false, time.Time{}, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", false, time.Time{}, nil, fmt.Errorf("stripe subscription retrieve failed: %s", resp.Status)
	}
	var parsed struct {
		Status            string            `json:"status"`
		Currency          string            `json:"currency"`
		CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
		CurrentPeriodEnd  int64             `json:"current_period_end"`
		Metadata          map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", false, time.Time{}, nil, err
	}
	t := time.Unix(parsed.CurrentPeriodEnd, 0).UTC()
	cur := strings.ToUpper(strings.TrimSpace(parsed.Currency))
	return parsed.Status, cur, parsed.CancelAtPeriodEnd, t, parsed.Metadata, nil
}

// RetrieveChargeMetadata fetches charge metadata and refunded amount.
func RetrieveChargeMetadata(chargeID string) (metadata map[string]string, amountRefundedMinor int64, currency string, err error) {
	sk := stripeSecretKey()
	if sk == "" || strings.TrimSpace(chargeID) == "" {
		return nil, 0, "", fmt.Errorf("missing stripe config or charge id")
	}
	req, err := http.NewRequest(http.MethodGet, stripeAPI+"/charges/"+url.PathEscape(chargeID), nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+sk)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, "", fmt.Errorf("stripe charge retrieve failed: %s: %s", resp.Status, truncate(string(body), 500))
	}
	var parsed struct {
		Metadata       map[string]string `json:"metadata"`
		AmountRefunded int64             `json:"amount_refunded"`
		Currency       string            `json:"currency"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, 0, "", err
	}
	return parsed.Metadata, parsed.AmountRefunded, strings.ToUpper(strings.TrimSpace(parsed.Currency)), nil
}

// WebhookConfigured reports whether webhook verification can run.
func WebhookConfigured() bool {
	return stripeWebhookSecret() != ""
}

// StripeWebhookSecret exposes secret for signature verification in handlers.
func StripeWebhookSecret() string {
	return stripeWebhookSecret()
}

// ParseStripeUnix parses Stripe unix timestamps from JSON numbers.
func ParseStripeUnix(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

// AmountMinor parses optional cents amount from env display strings (unused if prices come from Stripe).
func AmountMinor(amountStr string) (int64, bool) {
	a := strings.TrimSpace(amountStr)
	if a == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return 0, false
	}
	return int64(f * 100), true
}
