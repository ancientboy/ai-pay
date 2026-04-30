package http

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ai-pay-backend/internal/billing"
	"ai-pay-backend/internal/service"
)

type Server struct {
	svc              service.PaymentService
	nowFn            func() time.Time
	readyCheck       func(context.Context) error
	adminBearerToken string
	readonlyToken    string
	callbackToken    string
	callbackSignKey  string
	callbackNonceLRU *fixedWindowLimiter
	callbackDedupe   *callbackDedupeStore
	signatureMaxSkew time.Duration
	ipRateLimiter    rateLimiter
	agentRateLimiter rateLimiter
}

type contextKey string

const requestIDKey contextKey = "requestId"

func NewServer(svc service.PaymentService) *Server {
	return &Server{
		svc:              svc,
		nowFn:            time.Now,
		signatureMaxSkew: 5 * time.Minute,
		callbackNonceLRU: newFixedWindowLimiter(1, 10*time.Minute),
		callbackDedupe:   newCallbackDedupeStore(10 * time.Minute),
		ipRateLimiter:    newFixedWindowLimiter(120, time.Minute),
		agentRateLimiter: newFixedWindowLimiter(60, time.Minute),
	}
}

func NewServerWithReadiness(svc service.PaymentService, readyCheck func(context.Context) error) *Server {
	s := NewServer(svc)
	s.readyCheck = readyCheck
	return s
}

func (s *Server) SetAdminBearerToken(token string) {
	s.adminBearerToken = strings.TrimSpace(token)
}

func (s *Server) SetReadonlyBearerToken(token string) {
	s.readonlyToken = strings.TrimSpace(token)
}

func (s *Server) SetCallbackToken(token string) {
	s.callbackToken = strings.TrimSpace(token)
}

func (s *Server) SetCallbackSigningSecret(secret string) {
	s.callbackSignKey = strings.TrimSpace(secret)
}

func NewServerForTest(svc service.PaymentService, nowFn func() time.Time, ipLimit, agentLimit int) *Server {
	s := NewServer(svc)
	s.nowFn = nowFn
	s.ipRateLimiter = newFixedWindowLimiter(ipLimit, time.Minute)
	s.agentRateLimiter = newFixedWindowLimiter(agentLimit, time.Minute)
	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("POST /agent/did/register", s.handleRegisterAgent)
	mux.HandleFunc("POST /agent/did/verify", s.handleVerifyAgent)
	mux.HandleFunc("POST /agent/did/update", s.handleUpdateAgent)
	mux.HandleFunc("POST /account/create", s.handleCreateAccount)
	mux.HandleFunc("GET /agent/list", s.handleAgentList)
	mux.HandleFunc("POST /fund/recharge", s.handleRecharge)
	mux.HandleFunc("GET /fund/recharge/list", s.handleRechargeList)
	mux.HandleFunc("POST /authorize/payment/set", s.handleAuthorizeSet)
	mux.HandleFunc("POST /authorize/payment/update", s.handleAuthorizeUpdate)
	mux.HandleFunc("POST /authorize/freeze", s.handleAuthorizeFreeze)
	mux.HandleFunc("POST /authorize/activate", s.handleAuthorizeActivate)
	mux.HandleFunc("POST /payment/x402/pay", s.handlePay)
	mux.Handle("POST /payment/status/callback", s.withCallbackToken(http.HandlerFunc(s.handleStatusCallback)))
	mux.Handle("POST /payment/unfreeze", s.withAdminAuth(http.HandlerFunc(s.handleUnfreeze)))
	mux.Handle("POST /payment/refund", s.withAdminAuth(http.HandlerFunc(s.handleRefund)))
	mux.HandleFunc("GET /payment/status/query", s.handleStatus)
	mux.HandleFunc("GET /account/balance/query", s.handleBalance)
	mux.HandleFunc("GET /account/ledger/query", s.handleLedger)
	mux.Handle("GET /account/interest/query", s.withReadAuth(http.HandlerFunc(s.handleInterest)))
	mux.Handle("POST /account/va/topup/config", s.withAdminAuth(http.HandlerFunc(s.handleVATopupConfigSet)))
	mux.Handle("GET /account/va/topup/config", s.withReadAuth(http.HandlerFunc(s.handleVATopupConfigGet)))
	mux.Handle("POST /account/va/transfer", s.withAdminAuth(http.HandlerFunc(s.handleVATransfer)))
	mux.Handle("GET /account/va/transfer/list", s.withReadAuth(http.HandlerFunc(s.handleVATransferList)))
	mux.HandleFunc("GET /metrics/overview", s.handleOverviewMetrics)
	mux.Handle("GET /developer/api-keys", s.withReadAuth(http.HandlerFunc(s.handleAPIKeyList)))
	mux.Handle("POST /developer/api-keys", s.withAdminAuth(http.HandlerFunc(s.handleAPIKeyCreate)))
	mux.Handle("DELETE /developer/api-keys", s.withAdminAuth(http.HandlerFunc(s.handleAPIKeyDelete)))
	mux.Handle("GET /developer/webhooks", s.withReadAuth(http.HandlerFunc(s.handleWebhookList)))
	mux.Handle("POST /developer/webhooks", s.withAdminAuth(http.HandlerFunc(s.handleWebhookCreate)))
	mux.Handle("DELETE /developer/webhooks", s.withAdminAuth(http.HandlerFunc(s.handleWebhookDelete)))
	mux.Handle("GET /developer/webhook-deliveries", s.withReadAuth(http.HandlerFunc(s.handleWebhookDeliveryList)))
	mux.Handle("GET /developer/webhook-deliveries/stats", s.withReadAuth(http.HandlerFunc(s.handleWebhookDeliveryStats)))
	mux.Handle("POST /developer/webhook-deliveries/replay", s.withAdminAuth(http.HandlerFunc(s.handleWebhookDeliveryReplay)))
	mux.Handle("GET /developer/audit-logs", s.withReadAuth(http.HandlerFunc(s.handleAuditLogList)))
	mux.Handle("GET /developer/risk-config", s.withReadAuth(http.HandlerFunc(s.handleRiskConfigGet)))
	mux.Handle("POST /developer/risk-config", s.withAdminAuth(http.HandlerFunc(s.handleRiskConfigSet)))
	mux.Handle("GET /developer/channel-routes", s.withReadAuth(http.HandlerFunc(s.handleChannelRouteList)))
	mux.Handle("POST /developer/channel-routes", s.withAdminAuth(http.HandlerFunc(s.handleChannelRouteSet)))
	mux.Handle("DELETE /developer/channel-routes", s.withAdminAuth(http.HandlerFunc(s.handleChannelRouteDelete)))
	mux.Handle("POST /fund/transfer", s.withM6Funds(s.withAdminAuth(http.HandlerFunc(s.handleFundTransfer))))
	mux.Handle("POST /fund/withdraw", s.withM6Funds(s.withAdminAuth(http.HandlerFunc(s.handleFundWithdraw))))
	mux.Handle("POST /payment/debit/preview", s.withM6Funds(http.HandlerFunc(s.handleDebitPreview)))
	mux.Handle("POST /payment/x402/check", s.withM6Funds(http.HandlerFunc(s.handleX402Check)))
	mux.Handle("POST /payment/x402/transfer", s.withM6Funds(s.withAdminAuth(http.HandlerFunc(s.handleX402Transfer))))
	mux.Handle("POST /payment/refund/apply", s.withM6Funds(http.HandlerFunc(s.handleRefundApply)))
	mux.Handle("POST /payment/card/apply", s.withM7CardRisk(s.withAdminAuth(http.HandlerFunc(s.handleCardApply))))
	mux.Handle("POST /payment/card/pay", s.withM7CardRisk(http.HandlerFunc(s.handleCardPay)))
	mux.Handle("PUT /payment/card/manage", s.withM7CardRisk(s.withAdminAuth(http.HandlerFunc(s.handleCardManage))))
	mux.Handle("POST /risk/transaction/check", s.withM7CardRisk(s.withReadAuth(http.HandlerFunc(s.handleRiskTransactionCheck))))
	mux.Handle("POST /risk/kyc/verify", s.withM7CardRisk(http.HandlerFunc(s.handleRiskKYCVerify)))
	mux.Handle("GET /risk/audit/query", s.withM7CardRisk(s.withReadAuth(http.HandlerFunc(s.handleRiskAuditQuery))))
	mux.Handle("POST /wallet/bind", s.withM8SelfHosted(http.HandlerFunc(s.handleWalletBind)))
	mux.Handle("POST /wallet/unbind", s.withM8SelfHosted(http.HandlerFunc(s.handleWalletUnbind)))
	mux.Handle("POST /authorize/session/create", s.withM8SelfHosted(http.HandlerFunc(s.handleSessionCreate)))
	mux.Handle("POST /authorize/session/revoke", s.withM8SelfHosted(http.HandlerFunc(s.handleSessionRevoke)))
	mux.Handle("POST /payment/sign/request", s.withM8SelfHosted(http.HandlerFunc(s.handlePaymentSignRequest)))
	mux.Handle("POST /payment/sign/submit", s.withM8SelfHosted(http.HandlerFunc(s.handlePaymentSignSubmit)))
	mux.HandleFunc("GET /billing/plans", s.handleBillingPlans)
	mux.HandleFunc("GET /billing/provider/capabilities", s.handleBillingProviderCapabilities)
	mux.HandleFunc("GET /billing/subscription", s.handleBillingSubscription)
	mux.HandleFunc("GET /billing/checkout/sync", s.handleBillingCheckoutSync)
	mux.HandleFunc("POST /billing/checkout/create", s.handleBillingCheckoutCreate)
	mux.HandleFunc("POST /billing/intent/create", s.handleBillingCheckoutCreate)
	mux.HandleFunc("POST /billing/webhook/stripe", s.handleBillingStripeWebhook)
	return s.withRequestID(mux)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAPIError(w http.ResponseWriter, err *service.APIError) {
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"code":    err.Code,
		"message": err.Message,
	})
}

func writeInternalError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	log.Printf("requestId=%s op=%s err=%v", getRequestID(r.Context()), operation, err)
	writeJSON(w, http.StatusBadRequest, map[string]any{
		"code":    "PAY-010",
		"message": operation + " failed",
	})
}

type registerAgentReq struct {
	AgentDID  string `json:"agentDid"`
	DIDPubKey string `json:"didPubKey"`
}

type billingPlan struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	PriceMonthly   string   `json:"priceMonthly"`
	Currency       string   `json:"currency"`
	SupportedRails []string `json:"supportedRails"`
}

type billingProviderCapability struct {
	Provider      string   `json:"provider"`
	Capabilities  []string `json:"capabilities"`
	Enabled       bool     `json:"enabled"`
	CheckoutModes []string `json:"checkoutModes"`
}

type billingCheckoutCreateReq struct {
	PlanCode       string `json:"planCode"`
	PaymentRail    string `json:"paymentRail"`
	Currency       string `json:"currency"`
	Provider       string `json:"provider"`
	CustomerIDHint string `json:"customerIdHint"`
	Amount         string `json:"amount"`
}

func (s *Server) handleBillingPlans(w http.ResponseWriter, r *http.Request) {
	plans := []billingPlan{
		{
			Code:           "starter",
			Name:           "Starter",
			Description:    "For PoC and small pilot workloads",
			PriceMonthly:   "49",
			Currency:       "USD",
			SupportedRails: []string{"fiat", "stablecoin"},
		},
		{
			Code:           "growth",
			Name:           "Growth",
			Description:    "For production workloads and team collaboration",
			PriceMonthly:   "199",
			Currency:       "USD",
			SupportedRails: []string{"fiat", "stablecoin"},
		},
		{
			Code:           "enterprise",
			Name:           "Enterprise",
			Description:    "For compliance-heavy and high-volume organizations",
			PriceMonthly:   "custom",
			Currency:       "USD",
			SupportedRails: []string{"fiat", "stablecoin"},
		},
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": plans})
}

func envEnabled(name string, defaultEnabled bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if value == "" {
		return defaultEnabled
	}
	return value == "1" || value == "true" || value == "yes"
}

func (s *Server) handleBillingProviderCapabilities(w http.ResponseWriter, r *http.Request) {
	// Response shape matches console billing page: { capabilities: [...] }
	type capRow struct {
		Provider             string   `json:"provider"`
		Methods              []string `json:"methods"`
		Currencies           []string `json:"currencies"`
		SupportsSubscription bool     `json:"supportsSubscription"`
		DefaultMethod        string   `json:"defaultMethod"`
	}
	stripeReady := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("STRIPE_PRICE_STARTER")) != ""
	var caps []capRow
	if envEnabled("BILLING_PROVIDER_STRIPE_ENABLED", true) {
		caps = append(caps, capRow{
			Provider:             "stripe",
			Methods:              []string{"fiat_card", "fiat_bank"},
			Currencies:           []string{"USD", "EUR"},
			SupportsSubscription: true,
			DefaultMethod:        "fiat_card",
		})
	}
	if envEnabled("BILLING_PROVIDER_BRIDGE_ENABLED", true) {
		caps = append(caps, capRow{
			Provider:             "bridge",
			Methods:              []string{"stablecoin_transfer"},
			Currencies:           []string{"USDC", "USDT"},
			SupportsSubscription: false,
			DefaultMethod:        "stablecoin_transfer",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "0",
		"data": map[string]any{
			"capabilities":                  caps,
			"stripeCheckoutConfigured":      stripeReady,
			"stripeWebhookSecretConfigured": strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")) != "",
		},
	})
}

func (s *Server) handleBillingCheckoutSync(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing user context"})
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing session_id"})
		return
	}
	subID, custID, meta, err := billing.RetrieveCheckoutSession(sessionID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": err.Error()})
		return
	}
	uid := strings.TrimSpace(meta["user_id"])
	if uid != "" && uid != userID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "session user mismatch"})
		return
	}
	planCode := strings.TrimSpace(meta["plan_code"])
	if planCode == "" {
		planCode = "starter"
	}
	st := "active"
	currency := "USD"
	cancelEnd := false
	var periodEnd *time.Time
	if subID != "" {
		st2, cur, cancelAt, tEnd, smeta, err := billing.RetrieveSubscription(subID)
		if err == nil {
			st = st2
			cancelEnd = cancelAt
			if !tEnd.IsZero() {
				periodEnd = &tEnd
			}
			if cur != "" {
				currency = cur
			}
			if smeta != nil {
				if v := strings.TrimSpace(smeta["plan_code"]); v != "" {
					planCode = v
				}
			}
		}
	}
	_ = s.svc.UpsertBillingSubscription(service.BillingSubscriptionUpsert{
		UserID:                 userID,
		PlanCode:               planCode,
		Status:                 st,
		Currency:               currency,
		Provider:               "stripe",
		ProviderCustomerID:     custID,
		ProviderSubscriptionID: subID,
		CurrentPeriodEnd:       periodEnd,
		CancelAtPeriodEnd:      cancelEnd,
		RawEvent:               nil,
	})
	sub, ok := s.svc.GetBillingSubscription(userID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"subscription": nil}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"subscription": sub}})
}

func (s *Server) handleBillingSubscription(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing user context"})
		return
	}
	sub, ok := s.svc.GetBillingSubscription(userID)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"subscription": nil}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"subscription": sub}})
}

func resolveBillingProvider(rail string, preferred string) string {
	preferred = strings.TrimSpace(strings.ToLower(preferred))
	if preferred == "stripe" || preferred == "bridge" {
		return preferred
	}
	rail = strings.TrimSpace(strings.ToLower(rail))
	if rail == "fiat" {
		return "stripe"
	}
	return "bridge"
}

func (s *Server) handleBillingCheckoutCreate(w http.ResponseWriter, r *http.Request) {
	var req billingCheckoutCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing user context (X-User-Id)"})
		return
	}

	planCode := strings.TrimSpace(strings.ToLower(req.PlanCode))
	rail := strings.TrimSpace(strings.ToLower(req.PaymentRail))
	currency := strings.TrimSpace(strings.ToUpper(req.Currency))
	if planCode == "" || rail == "" || currency == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing required fields"})
		return
	}
	if rail != "fiat" && rail != "stablecoin" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid payment rail"})
		return
	}
	provider := resolveBillingProvider(rail, req.Provider)
	if provider == "stripe" && !envEnabled("BILLING_PROVIDER_STRIPE_ENABLED", true) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "stripe provider disabled"})
		return
	}
	if provider == "bridge" && !envEnabled("BILLING_PROVIDER_BRIDGE_ENABLED", true) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "bridge provider disabled"})
		return
	}

	checkoutID := fmt.Sprintf("chk_%d", time.Now().UnixNano())
	mockURL := fmt.Sprintf(
		"https://checkout.mock.%s.local/%s?plan=%s&rail=%s&currency=%s",
		provider,
		checkoutID,
		planCode,
		rail,
		currency,
	)

	// --- Stripe subscription (fiat USD path) ---
	if provider == "stripe" && rail == "fiat" && currency == "USD" {
		priceID, hasPrice := billing.PriceIDForPlan(planCode)
		if strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")) != "" && hasPrice {
			base := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_PUBLIC_URL")), "/")
			if base == "" {
				base = "http://127.0.0.1:3000"
			}
			successURL := base + "/billing?checkout=success&session_id={CHECKOUT_SESSION_ID}"
			cancelURL := base + "/billing?checkout=cancel"
			meta := map[string]string{
				"plan_code": planCode,
				"user_id":   userID,
			}
			res, err := billing.CreateSubscriptionCheckout(priceID, successURL, cancelURL, meta)
			if err != nil {
				log.Printf("billing stripe checkout error: %v", err)
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": err.Error()})
				return
			}
			var amtMinor *int64
			if m, ok := billing.AmountMinor(strings.TrimSpace(req.Amount)); ok {
				amtMinor = &m
			}
			_ = s.svc.RecordBillingCheckoutSession(service.BillingCheckoutSessionInput{
				LocalID:           checkoutID,
				UserID:            userID,
				Provider:          "stripe",
				PlanCode:          planCode,
				PaymentRail:       rail,
				Currency:          currency,
				AmountMinor:       amtMinor,
				Status:            "open",
				CheckoutURL:       res.URL,
				ProviderSessionID: res.ID,
				Metadata:          meta,
			})
			writeJSON(w, http.StatusOK, map[string]any{
				"code": "0",
				"data": map[string]any{
					"checkoutId":      checkoutID,
					"provider":        "stripe",
					"paymentRail":     rail,
					"currency":        currency,
					"status":          res.Status,
					"checkoutURL":     res.URL,
					"providerSession": res.ID,
					"customerHint":    strings.TrimSpace(req.CustomerIDHint),
					"requestedPlan":   planCode,
					"checkoutMode":    "live",
				},
			})
			return
		}
		if strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")) != "" && !hasPrice {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"code":    "PAY-010",
				"message": "set STRIPE_PRICE_" + strings.ToUpper(planCode) + " for this plan or choose starter/growth",
			})
			return
		}
	}

	// --- Fallback mock checkout (dev / missing keys / bridge) ---
	_ = s.svc.RecordBillingCheckoutSession(service.BillingCheckoutSessionInput{
		LocalID:     checkoutID,
		UserID:      userID,
		Provider:    provider,
		PlanCode:    planCode,
		PaymentRail: rail,
		Currency:    currency,
		Status:      "mock_open",
		CheckoutURL: mockURL,
		Metadata: map[string]string{
			"plan_code": planCode,
			"user_id":   userID,
			"mode":      "mock",
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "0",
		"data": map[string]any{
			"checkoutId":    checkoutID,
			"provider":      provider,
			"paymentRail":   rail,
			"currency":      currency,
			"status":        "PENDING",
			"checkoutURL":   mockURL,
			"customerHint":  strings.TrimSpace(req.CustomerIDHint),
			"requestedPlan": planCode,
			"checkoutMode":  "mock",
		},
	})
}

func (s *Server) handleBillingStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": "PAY-010", "message": "method not allowed"})
		return
	}
	sig := r.Header.Get("Stripe-Signature")
	payload, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "read body failed"})
		return
	}
	if err := billing.VerifyWebhookSignature(payload, sig); err != nil {
		log.Printf("stripe webhook verify: %v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid signature"})
		return
	}
	var envelope struct {
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid json"})
		return
	}
	rawObj := envelope.Data.Object
	if len(rawObj) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"received": true})
		return
	}

	switch envelope.Type {
	case "checkout.session.completed":
		var sess struct {
			ID            string            `json:"id"`
			Mode          string            `json:"mode"`
			Subscription  json.RawMessage   `json:"subscription"`
			Customer      json.RawMessage   `json:"customer"`
			Metadata      map[string]string `json:"metadata"`
			PaymentStatus string            `json:"payment_status"`
		}
		if err := json.Unmarshal(rawObj, &sess); err != nil {
			break
		}
		userID := strings.TrimSpace(sess.Metadata["user_id"])
		planCode := strings.TrimSpace(sess.Metadata["plan_code"])
		if userID == "" {
			break
		}
		subID := ""
		if len(sess.Subscription) > 0 && sess.Subscription[0] == '"' {
			_ = json.Unmarshal(sess.Subscription, &subID)
		} else if len(sess.Subscription) > 0 {
			var so struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(sess.Subscription, &so) == nil {
				subID = so.ID
			}
		}
		custID := ""
		if len(sess.Customer) > 0 && sess.Customer[0] == '"' {
			_ = json.Unmarshal(sess.Customer, &custID)
		} else if len(sess.Customer) > 0 {
			var co struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(sess.Customer, &co) == nil {
				custID = co.ID
			}
		}
		if planCode == "" {
			planCode = "starter"
		}
		// Refresh from Stripe if subscription id present
		var periodEnd *time.Time
		st := "active"
		currency := "USD"
		cancelEnd := false
		if subID != "" {
			st2, billCur, cancelAt, tEnd, meta, err := billing.RetrieveSubscription(subID)
			if err == nil {
				st = st2
				cancelEnd = cancelAt
				if !tEnd.IsZero() {
					periodEnd = &tEnd
				}
				if billCur != "" {
					currency = billCur
				}
				if meta != nil {
					if v := strings.TrimSpace(meta["plan_code"]); v != "" {
						planCode = v
					}
				}
			}
		}
		_ = s.svc.UpsertBillingSubscription(service.BillingSubscriptionUpsert{
			UserID:                 userID,
			PlanCode:               planCode,
			Status:                 st,
			Currency:               currency,
			Provider:               "stripe",
			ProviderCustomerID:     custID,
			ProviderSubscriptionID: subID,
			CurrentPeriodEnd:       periodEnd,
			CancelAtPeriodEnd:      cancelEnd,
			RawEvent:               payload,
		})

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub struct {
			ID                   string `json:"id"`
			Status               string `json:"status"`
			Currency             string `json:"currency"`
			CancelAtPeriodEnd    bool   `json:"cancel_at_period_end"`
			CurrentPeriodEnd     int64  `json:"current_period_end"`
			Metadata             map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(rawObj, &sub); err != nil {
			break
		}
		userID := strings.TrimSpace(sub.Metadata["user_id"])
		planCode := strings.TrimSpace(sub.Metadata["plan_code"])
		if userID == "" {
			break
		}
		if planCode == "" {
			planCode = "starter"
		}
		cur := strings.ToUpper(strings.TrimSpace(sub.Currency))
		if cur == "" {
			cur = "USD"
		}
		var pe *time.Time
		if sub.CurrentPeriodEnd > 0 {
			t := time.Unix(sub.CurrentPeriodEnd, 0).UTC()
			pe = &t
		}
		st := sub.Status
		if envelope.Type == "customer.subscription.deleted" {
			st = "canceled"
		}
		_ = s.svc.UpsertBillingSubscription(service.BillingSubscriptionUpsert{
			UserID:                 userID,
			PlanCode:               planCode,
			Status:                 st,
			Currency:               cur,
			Provider:               "stripe",
			ProviderSubscriptionID: sub.ID,
			CurrentPeriodEnd:       pe,
			CancelAtPeriodEnd:      sub.CancelAtPeriodEnd,
			RawEvent:               payload,
		})
	default:
		// ignore
	}
	writeJSON(w, http.StatusOK, map[string]any{"received": true})
}

func (s *Server) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req registerAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || strings.TrimSpace(req.DIDPubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !isBase64Ed25519PubKey(req.DIDPubKey) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid did pub key"})
		return
	}
	agent := s.svc.RegisterAgent(req.AgentDID)
	if err := s.svc.SetAgentPublicKey(req.AgentDID, req.DIDPubKey); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "save did pub key failed"})
		return
	}
	agent.DIDPubKey = req.DIDPubKey
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": agent})
}

type createAccountReq struct {
	AgentDID string `json:"agentDid"`
}

type verifyAgentReq struct {
	AgentDID  string `json:"agentDid"`
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

type updateAgentReq struct {
	AgentDID       string `json:"agentDid"`
	NewDIDPubKey   string `json:"newDidPubKey"`
	SignTimestamp  string `json:"signTimestamp"`
	ProofSignature string `json:"proofSignature"`
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	acc := s.svc.CreateAccount(req.AgentDID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": acc})
}

func (s *Server) handleVerifyAgent(w http.ResponseWriter, r *http.Request) {
	var req verifyAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Message) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.VerifyAgentSignature(req.AgentDID, req.Message, req.Signature); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"verified": true}})
}

func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	var req updateAgentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.NewDIDPubKey) == "" ||
		strings.TrimSpace(req.SignTimestamp) == "" ||
		strings.TrimSpace(req.ProofSignature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !isBase64Ed25519PubKey(req.NewDIDPubKey) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid did pub key"})
		return
	}
	if !s.validateSignatureTimestamp(req.SignTimestamp) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	proofMessage := buildUpdateKeyProofPayload(req.AgentDID, req.NewDIDPubKey, req.SignTimestamp)
	if err := s.svc.UpdateAgentPublicKey(req.AgentDID, req.NewDIDPubKey, proofMessage, req.ProofSignature); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAgentList(w http.ResponseWriter, r *http.Request) {
	list := s.svc.ListAgents()
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": list})
}

type rechargeReq struct {
	VAAccountID string `json:"vaAccountId"`
	VACardNo    string `json:"vaCardNo"`
	Amount      string `json:"amount"`
}

func (s *Server) handleRecharge(w http.ResponseWriter, r *http.Request) {
	var req rechargeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	accountRef := strings.TrimSpace(req.VAAccountID)
	if accountRef == "" {
		accountRef = strings.TrimSpace(req.VACardNo)
	}
	if accountRef == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Recharge(accountRef, req.Amount, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "recharge", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleRechargeList(w http.ResponseWriter, r *http.Request) {
	va := strings.TrimSpace(r.URL.Query().Get("accountId"))
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	list := s.svc.ListRecharges(va, limit)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": list})
}

type authorizeReq struct {
	AgentDID    string   `json:"agentDid"`
	SingleLimit string   `json:"singleLimit"`
	DailyLimit  string   `json:"dailyLimit"`
	Whitelist   []string `json:"whitelist"`
}

type freezeAuthorizeReq struct {
	AgentDID string `json:"agentDid"`
}

func (s *Server) handleAuthorizeSet(w http.ResponseWriter, r *http.Request) {
	var req authorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || !isPositiveDecimal(req.SingleLimit) || !isPositiveDecimal(req.DailyLimit) || len(req.Whitelist) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.SetAuthorizeRule(req.AgentDID, req.SingleLimit, req.DailyLimit, req.Whitelist); err != nil {
		writeInternalError(w, r, "set authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_set", req.AgentDID, map[string]any{"singleLimit": req.SingleLimit, "dailyLimit": req.DailyLimit})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuthorizeUpdate(w http.ResponseWriter, r *http.Request) {
	var req authorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentDID == "" || !isPositiveDecimal(req.SingleLimit) || !isPositiveDecimal(req.DailyLimit) || len(req.Whitelist) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.UpdateAuthorizeRule(req.AgentDID, req.SingleLimit, req.DailyLimit, req.Whitelist); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "update authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_update", req.AgentDID, map[string]any{"singleLimit": req.SingleLimit, "dailyLimit": req.DailyLimit})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuthorizeFreeze(w http.ResponseWriter, r *http.Request) {
	var req freezeAuthorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AgentDID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.FreezeAuthorizeRule(req.AgentDID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "freeze authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_freeze", req.AgentDID, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuthorizeActivate(w http.ResponseWriter, r *http.Request) {
	var req freezeAuthorizeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AgentDID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.ActivateAuthorizeRule(req.AgentDID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "activate authorize rule", err)
		return
	}
	s.appendAuditLog(r, "authorize_activate", req.AgentDID, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type payReq struct {
	PayerDID   string `json:"payerDid"`
	MerchantID string `json:"merchantId"`
	Amount     string `json:"amount"`
	Signature  string `json:"signature"`
}

type statusCallbackReq struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
}

type txActionReq struct {
	TransactionID string `json:"transactionId"`
}

type vaTopupConfigReq struct {
	AccountID        string `json:"accountId"`
	AutoTopupEnabled bool   `json:"autoTopupEnabled"`
	ThresholdAmount  string `json:"thresholdAmount"`
	TargetAmount     string `json:"targetAmount"`
}

type vaTransferReq struct {
	FromAccountID string `json:"fromAccountId"`
	ToAccountID   string `json:"toAccountId"`
	Amount        string `json:"amount"`
}

func (s *Server) handlePay(w http.ResponseWriter, r *http.Request) {
	var req payReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if req.PayerDID == "" || req.MerchantID == "" || req.Signature == "" || !isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	ipKey := clientIP(r)
	agentKey := "agent:" + req.PayerDID
	now := s.nowFn().UTC()
	if !s.ipRateLimiter.allow("ip:"+ipKey, now) || !s.agentRateLimiter.allow(agentKey, now) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"code": "PAY-010", "message": "rate limit exceeded"})
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(req.PayerDID)
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := buildPaySignaturePayload(req.PayerDID, req.MerchantID, req.Amount, idem, r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	log.Printf("requestId=%s pay request did=%s merchant=%s amount=%s ip=%s", getRequestID(r.Context()), maskDID(req.PayerDID), req.MerchantID, req.Amount, ipKey)
	resp, apiErr := s.svc.Pay(service.PayRequest{
		PayerDID:       req.PayerDID,
		MerchantID:     req.MerchantID,
		Amount:         req.Amount,
		IdempotencyKey: idem,
		Signature:      req.Signature,
	})
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": resp})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("transactionId")
	tx, err := s.svc.QueryStatus(txID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-010", "message": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": tx})
}

func (s *Server) handleStatusCallback(w http.ResponseWriter, r *http.Request) {
	var req statusCallbackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	switch status {
	case "SETTLED":
		if err := s.svc.ResolveSettling(req.TransactionID, true); err != nil {
			writeInternalError(w, r, "resolve callback status", err)
			return
		}
	case "FAILED":
		if err := s.svc.ResolveSettling(req.TransactionID, false); err != nil {
			writeInternalError(w, r, "resolve callback status", err)
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid status"})
		return
	}
	log.Printf("requestId=%s callback resolved tx=%s status=%s", getRequestID(r.Context()), req.TransactionID, status)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleUnfreeze(w http.ResponseWriter, r *http.Request) {
	var req txActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Unfreeze(req.TransactionID, idem); err != nil {
		writeInternalError(w, r, "unfreeze", err)
		return
	}
	log.Printf("requestId=%s unfreeze tx=%s", getRequestID(r.Context()), req.TransactionID)
	s.appendAuditLog(r, "payment_unfreeze", req.TransactionID, map[string]any{"idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleRefund(w http.ResponseWriter, r *http.Request) {
	var req txActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.Refund(req.TransactionID, idem); err != nil {
		writeInternalError(w, r, "refund", err)
		return
	}
	log.Printf("requestId=%s refund tx=%s", getRequestID(r.Context()), req.TransactionID)
	s.appendAuditLog(r, "payment_refund", req.TransactionID, map[string]any{"idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	va := r.URL.Query().Get("accountId")
	balance, err := s.svc.BalanceByVA(va)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-010", "message": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"balance": balance}})
}

func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request) {
	va := r.URL.Query().Get("accountId")
	if va == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	ledger := s.svc.LedgerByVA(va)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": ledger})
}

func (s *Server) handleOverviewMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.svc.OverviewMetrics()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": "PAY-010", "message": "query metrics failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": metrics})
}

func (s *Server) handleInterest(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	quote, err := s.svc.QueryInterest(accountID)
	if err != nil {
		writeInternalError(w, r, "query interest", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": quote})
}

func (s *Server) handleVATopupConfigSet(w http.ResponseWriter, r *http.Request) {
	var req vaTopupConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AccountID) == "" ||
		!isNonNegativeDecimal(req.ThresholdAmount) ||
		!isPositiveDecimal(req.TargetAmount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.SetVATopupConfig(req.AccountID, req.AutoTopupEnabled, req.ThresholdAmount, req.TargetAmount)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set va topup config", err)
		return
	}
	s.appendAuditLog(r, "va_topup_config_set", req.AccountID, map[string]any{
		"autoTopupEnabled": req.AutoTopupEnabled,
		"thresholdAmount":  req.ThresholdAmount,
		"targetAmount":     req.TargetAmount,
	})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleVATopupConfigGet(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	if accountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.GetVATopupConfig(accountID)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "query va topup config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleVATransfer(w http.ResponseWriter, r *http.Request) {
	var req vaTransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.FromAccountID) == "" ||
		strings.TrimSpace(req.ToAccountID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.TransferVA(req.FromAccountID, req.ToAccountID, req.Amount, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "va transfer", err)
		return
	}
	s.appendAuditLog(r, "va_transfer", req.FromAccountID+"->"+req.ToAccountID, map[string]any{"amount": req.Amount, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleVATransferList(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.URL.Query().Get("accountId"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	startTime := strings.TrimSpace(r.URL.Query().Get("startTime"))
	endTime := strings.TrimSpace(r.URL.Query().Get("endTime"))
	if startTime != "" {
		if _, err := time.Parse(time.RFC3339, startTime); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid startTime"})
			return
		}
	}
	if endTime != "" {
		if _, err := time.Parse(time.RFC3339, endTime); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid endTime"})
			return
		}
	}
	limit := 20
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items := s.svc.ListVATransfers(accountID, status, startTime, endTime, limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

type fundTransferReq struct {
	FromAccountID string `json:"fromAccountId"`
	ToAccountID   string `json:"toAccountId"`
	Amount        string `json:"amount"`
}

func (s *Server) handleFundTransfer(w http.ResponseWriter, r *http.Request) {
	var req fundTransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.FromAccountID) == "" ||
		strings.TrimSpace(req.ToAccountID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.TransferFunds(req.FromAccountID, req.ToAccountID, req.Amount, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "fund transfer", err)
		return
	}
	log.Printf("requestId=%s fund_transfer from=%s to=%s amount=%s", getRequestID(r.Context()), req.FromAccountID, req.ToAccountID, req.Amount)
	s.appendAuditLog(r, "fund_transfer", req.FromAccountID+"->"+req.ToAccountID, map[string]any{"amount": req.Amount, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type fundWithdrawReq struct {
	VAAccountID     string `json:"vaAccountId"`
	Amount          string `json:"amount"`
	Rail            string `json:"rail"`
	DestinationHint string `json:"destinationHint"`
}

func (s *Server) handleFundWithdraw(w http.ResponseWriter, r *http.Request) {
	var req fundWithdrawReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.VAAccountID) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Rail) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	rec, err := s.svc.WithdrawFunds(req.VAAccountID, req.Amount, req.Rail, req.DestinationHint, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "fund withdraw", err)
		return
	}
	log.Printf("requestId=%s fund_withdraw id=%s va=%s amount=%s rail=%s", getRequestID(r.Context()), rec.WithdrawID, rec.VAAccountID, req.Amount, rec.Rail)
	s.appendAuditLog(r, "fund_withdraw", rec.WithdrawID, map[string]any{"vaAccountId": rec.VAAccountID, "amount": req.Amount, "rail": rec.Rail, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type debitPreviewReq struct {
	AgentDID   string `json:"agentDid"`
	MerchantID string `json:"merchantId"`
	Amount     string `json:"amount"`
	Signature  string `json:"signature"`
}

func (s *Server) handleDebitPreview(w http.ResponseWriter, r *http.Request) {
	var req debitPreviewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(req.AgentDID)
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := []byte("debit_preview|" + req.AgentDID + "|" + req.MerchantID + "|" + req.Amount + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	pv, err := s.svc.DebitPreview(req.AgentDID, req.MerchantID, req.Amount)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "debit preview", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": pv})
}

type x402CheckReq struct {
	TransactionID string `json:"transactionId"`
}

func (s *Server) handleX402Check(w http.ResponseWriter, r *http.Request) {
	var req x402CheckReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.TransactionID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	data, err := s.svc.CheckX402Settlement(req.TransactionID)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "x402 check", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": data})
}

type x402TransferReq struct {
	VAAccountID            string `json:"vaAccountId"`
	ToAddress              string `json:"toAddress"`
	Amount                 string `json:"amount"`
	ReferenceTransactionID string `json:"referenceTransactionId"`
}

func (s *Server) handleX402Transfer(w http.ResponseWriter, r *http.Request) {
	var req x402TransferReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.VAAccountID) == "" ||
		strings.TrimSpace(req.ToAddress) == "" ||
		strings.TrimSpace(req.ReferenceTransactionID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if err := s.svc.TransferX402Outbound(req.VAAccountID, req.ToAddress, req.Amount, req.ReferenceTransactionID, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "x402 transfer", err)
		return
	}
	log.Printf("requestId=%s x402_outbound va=%s ref=%s amount=%s", getRequestID(r.Context()), req.VAAccountID, req.ReferenceTransactionID, req.Amount)
	s.appendAuditLog(r, "payment_x402_transfer", req.ReferenceTransactionID, map[string]any{"vaAccountId": req.VAAccountID, "toAddress": req.ToAddress, "amount": req.Amount, "idempotencyKey": idem})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type refundApplyReq struct {
	AgentDID      string `json:"agentDid"`
	TransactionID string `json:"transactionId"`
	Reason        string `json:"reason"`
	Signature     string `json:"signature"`
}

func (s *Server) handleRefundApply(w http.ResponseWriter, r *http.Request) {
	var req refundApplyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.TransactionID) == "" ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := []byte("refund_apply|" + strings.TrimSpace(req.TransactionID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.RefundApply(req.TransactionID, strings.TrimSpace(req.Reason), idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "refund apply", err)
		return
	}
	log.Printf("requestId=%s refund_apply tx=%s did=%s", getRequestID(r.Context()), req.TransactionID, req.AgentDID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type cardApplyReq struct {
	AgentDID    string `json:"agentDid"`
	VAAccountID string `json:"vaAccountId"`
	CreditLimit string `json:"creditLimit"`
}

func (s *Server) handleCardApply(w http.ResponseWriter, r *http.Request) {
	var req cardApplyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.VAAccountID) == "" ||
		!isPositiveDecimal(req.CreditLimit) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	rec, err := s.svc.ApplyVirtualCard(req.AgentDID, req.VAAccountID, req.CreditLimit, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "card apply", err)
		return
	}
	log.Printf("requestId=%s card_apply id=%s", getRequestID(r.Context()), rec.CardID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type cardPayReq struct {
	AgentDID   string `json:"agentDid"`
	CardID     string `json:"cardId"`
	MerchantID string `json:"merchantId"`
	Amount     string `json:"amount"`
	Signature  string `json:"signature"`
}

func (s *Server) handleCardPay(w http.ResponseWriter, r *http.Request) {
	var req cardPayReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.CardID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("card_pay|" + req.AgentDID + "|" + req.CardID + "|" + req.MerchantID + "|" + req.Amount + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	txID, err := s.svc.PayVirtualCard(req.AgentDID, req.CardID, req.MerchantID, req.Amount, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "card pay", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": map[string]any{"transactionId": txID}})
}

type cardManageReq struct {
	CardID       string `json:"cardId"`
	Operation    string `json:"operation"`
	AdjustAmount string `json:"adjustAmount"`
}

func (s *Server) handleCardManage(w http.ResponseWriter, r *http.Request) {
	var req cardManageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.CardID) == "" ||
		strings.TrimSpace(req.Operation) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	rec, err := s.svc.ManageVirtualCard(req.CardID, req.Operation, req.AdjustAmount)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "card manage", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type riskTxnCheckReq struct {
	AgentDID      string `json:"agentDid"`
	MerchantID    string `json:"merchantId"`
	Amount        string `json:"amount"`
	TransactionID string `json:"transactionId"`
}

func (s *Server) handleRiskTransactionCheck(w http.ResponseWriter, r *http.Request) {
	var req riskTxnCheckReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		!isPositiveDecimal(req.Amount) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	out, err := s.svc.RiskTransactionCheck(req.AgentDID, req.MerchantID, req.Amount, strings.TrimSpace(req.TransactionID))
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "risk transaction check", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": out})
}

type riskKYCReq struct {
	AgentDID           string `json:"agentDid"`
	DocumentReference  string `json:"documentReference"`
	Signature          string `json:"signature"`
}

func (s *Server) handleRiskKYCVerify(w http.ResponseWriter, r *http.Request) {
	var req riskKYCReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("kyc_verify|" + strings.TrimSpace(req.AgentDID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	st, err := s.svc.RiskKYCVerify(req.AgentDID, strings.TrimSpace(req.DocumentReference), idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "risk kyc verify", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": st})
}

func (s *Server) handleRiskAuditQuery(w http.ResponseWriter, r *http.Request) {
	agentDID := strings.TrimSpace(r.URL.Query().Get("agentDid"))
	merchantID := strings.TrimSpace(r.URL.Query().Get("merchantId"))
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	items := s.svc.RiskAuditQuery(agentDID, merchantID, limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

type walletBindReq struct {
	AgentDID      string `json:"agentDid"`
	WalletAddress string `json:"walletAddress"`
	Label         string `json:"label"`
	Signature     string `json:"signature"`
}

func (s *Server) handleWalletBind(w http.ResponseWriter, r *http.Request) {
	var req walletBindReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.WalletAddress) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("wallet_bind|" + strings.TrimSpace(req.AgentDID) + "|" + strings.TrimSpace(req.WalletAddress) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.BindWallet(req.AgentDID, req.WalletAddress, req.Label); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "wallet bind", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type walletUnbindReq struct {
	AgentDID  string `json:"agentDid"`
	Signature string `json:"signature"`
}

func (s *Server) handleWalletUnbind(w http.ResponseWriter, r *http.Request) {
	var req walletUnbindReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("wallet_unbind|" + strings.TrimSpace(req.AgentDID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.UnbindWallet(req.AgentDID); err != nil {
		writeInternalError(w, r, "wallet unbind", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type sessionCreateReq struct {
	AgentDID    string `json:"agentDid"`
	TTLMinutes  int    `json:"ttlMinutes"`
	Signature   string `json:"signature"`
}

func (s *Server) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var req sessionCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("session_create|" + strings.TrimSpace(req.AgentDID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	st, err := s.svc.CreateAuthSession(req.AgentDID, req.TTLMinutes, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "session create", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": st})
}

type sessionRevokeReq struct {
	AgentDID  string `json:"agentDid"`
	SessionID string `json:"sessionId"`
	Signature string `json:"signature"`
}

func (s *Server) handleSessionRevoke(w http.ResponseWriter, r *http.Request) {
	var req sessionRevokeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("session_revoke|" + strings.TrimSpace(req.AgentDID) + "|" + strings.TrimSpace(req.SessionID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	if err := s.svc.RevokeAuthSession(req.AgentDID, req.SessionID, idem); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "session revoke", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

type paymentSignRequestHTTP struct {
	AgentDID   string `json:"agentDid"`
	MerchantID string `json:"merchantId"`
	Amount     string `json:"amount"`
	SessionID  string `json:"sessionId"`
	Signature  string `json:"signature"`
}

func (s *Server) handlePaymentSignRequest(w http.ResponseWriter, r *http.Request) {
	var req paymentSignRequestHTTP
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.AgentDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-008", "message": "missing idempotency key"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.AgentDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	payload := []byte("sign_request|" + strings.TrimSpace(req.AgentDID) + "|" + strings.TrimSpace(req.MerchantID) + "|" + strings.TrimSpace(req.Amount) + "|" + strings.TrimSpace(req.SessionID) + "|" + idem + "|" + r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, payload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	rec, err := s.svc.RequestPaymentSign(req.AgentDID, req.MerchantID, req.Amount, req.SessionID, idem)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "payment sign request", err)
		return
	}
	log.Printf("requestId=%s payment_sign_request id=%s", getRequestID(r.Context()), rec.SignID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": rec})
}

type paymentSignSubmitReq struct {
	SignID       string `json:"signId"`
	PayerDID     string `json:"payerDid"`
	MerchantID   string `json:"merchantId"`
	Amount       string `json:"amount"`
	IdempotencyKey string `json:"idempotencyKey"`
	Signature    string `json:"signature"`
}

func (s *Server) handlePaymentSignSubmit(w http.ResponseWriter, r *http.Request) {
	var req paymentSignSubmitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		strings.TrimSpace(req.SignID) == "" ||
		strings.TrimSpace(req.PayerDID) == "" ||
		strings.TrimSpace(req.MerchantID) == "" ||
		!isPositiveDecimal(req.Amount) ||
		strings.TrimSpace(req.Signature) == "" ||
		strings.TrimSpace(req.IdempotencyKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if !s.validateSignatureTimestamp(r.Header.Get("X-Sign-Timestamp")) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid or expired signature timestamp"})
		return
	}
	pubKey, err := s.svc.AgentPublicKey(strings.TrimSpace(req.PayerDID))
	if err != nil || strings.TrimSpace(pubKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "missing did public key"})
		return
	}
	signPayload := buildPaySignaturePayload(req.PayerDID, req.MerchantID, req.Amount, strings.TrimSpace(req.IdempotencyKey), r.Header.Get("X-Sign-Timestamp"))
	if !verifyDIDSignature(pubKey, req.Signature, signPayload) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-001", "message": "invalid did signature"})
		return
	}
	resp, apiErr := s.svc.SubmitSignedPayment(strings.TrimSpace(req.SignID), service.PayRequest{
		PayerDID:       strings.TrimSpace(req.PayerDID),
		MerchantID:     strings.TrimSpace(req.MerchantID),
		Amount:         strings.TrimSpace(req.Amount),
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		Signature:      strings.TrimSpace(req.Signature),
	})
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	log.Printf("requestId=%s payment_sign_submit signId=%s tx=%s", getRequestID(r.Context()), req.SignID, resp.TransactionID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": resp})
}

func (s *Server) handleAPIKeyList(w http.ResponseWriter, r *http.Request) {
	keys := s.svc.ListAPIKeys()
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": keys})
}

type createAPIKeyReq struct {
	Name string `json:"name"`
}

func (s *Server) handleAPIKeyCreate(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.CreateAPIKey(strings.TrimSpace(req.Name))
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "create api key", err)
		return
	}
	log.Printf("requestId=%s api key created id=%s", getRequestID(r.Context()), item.ID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleAPIKeyDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.DeleteAPIKey(id); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "delete api key", err)
		return
	}
	log.Printf("requestId=%s api key deleted id=%s", getRequestID(r.Context()), id)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleWebhookList(w http.ResponseWriter, r *http.Request) {
	items := s.svc.ListWebhooks()
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

type createWebhookReq struct {
	URL   string `json:"url"`
	Event string `json:"event"`
}

func (s *Server) handleWebhookCreate(w http.ResponseWriter, r *http.Request) {
	var req createWebhookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.CreateWebhook(strings.TrimSpace(req.URL), strings.TrimSpace(req.Event))
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "create webhook", err)
		return
	}
	log.Printf("requestId=%s webhook created id=%s", getRequestID(r.Context()), item.ID)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleWebhookDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.DeleteWebhook(id); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "delete webhook", err)
		return
	}
	log.Printf("requestId=%s webhook deleted id=%s", getRequestID(r.Context()), id)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleWebhookDeliveryList(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	event := strings.TrimSpace(r.URL.Query().Get("event"))
	webhookID := strings.TrimSpace(r.URL.Query().Get("webhookId"))
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items, err := s.svc.ListWebhookDeliveries(status, event, webhookID, limit, offset)
	if err != nil {
		writeInternalError(w, r, "list webhook deliveries", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

func (s *Server) handleWebhookDeliveryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.svc.WebhookDeliveryStats()
	if err != nil {
		writeInternalError(w, r, "query webhook delivery stats", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": stats})
}

type replayWebhookDeliveryReq struct {
	ID int64 `json:"id"`
}

type riskConfigSetReq struct {
	Enabled           bool     `json:"enabled"`
	SingleAmountLimit string   `json:"singleAmountLimit"`
	BlockedMerchants  []string `json:"blockedMerchants"`
}

type channelRouteSetReq struct {
	MerchantID string `json:"merchantId"`
	Mode       string `json:"mode"`
}

func (s *Server) handleWebhookDeliveryReplay(w http.ResponseWriter, r *http.Request) {
	var req replayWebhookDeliveryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.ReplayWebhookDelivery(req.ID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "replay webhook delivery", err)
		return
	}
	log.Printf("requestId=%s webhook delivery replay id=%d", getRequestID(r.Context()), req.ID)
	s.appendAuditLog(r, "webhook_delivery_replay", strconv.FormatInt(req.ID, 10), map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleAuditLogList(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	items := s.svc.ListAuditLogs(action, resource, limit, offset)
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": items})
}

func (s *Server) handleRiskConfigGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.GetRiskConfig()})
}

func (s *Server) handleRiskConfigSet(w http.ResponseWriter, r *http.Request) {
	var req riskConfigSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !isPositiveDecimal(req.SingleAmountLimit) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.SetRiskConfig(req.Enabled, req.SingleAmountLimit, req.BlockedMerchants)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set risk config", err)
		return
	}
	s.appendAuditLog(r, "risk_config_set", "global", map[string]any{
		"enabled":           req.Enabled,
		"singleAmountLimit": req.SingleAmountLimit,
		"blockedCount":      len(req.BlockedMerchants),
	})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleChannelRouteList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": s.svc.ListChannelRoutes()})
}

func (s *Server) handleChannelRouteSet(w http.ResponseWriter, r *http.Request) {
	var req channelRouteSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	item, err := s.svc.SetChannelRoute(req.MerchantID, req.Mode)
	if err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "set channel route", err)
		return
	}
	s.appendAuditLog(r, "channel_route_set", req.MerchantID, map[string]any{"mode": req.Mode})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "data": item})
}

func (s *Server) handleChannelRouteDelete(w http.ResponseWriter, r *http.Request) {
	merchantID := strings.TrimSpace(r.URL.Query().Get("merchantId"))
	if merchantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
		return
	}
	if err := s.svc.DeleteChannelRoute(merchantID); err != nil {
		if apiErr, ok := err.(*service.APIError); ok {
			writeAPIError(w, apiErr)
			return
		}
		writeInternalError(w, r, "delete channel route", err)
		return
	}
	s.appendAuditLog(r, "channel_route_delete", merchantID, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code": "0",
		"data": map[string]any{
			"status":    "ok",
			"requestId": getRequestID(r.Context()),
			"time":      s.nowFn().UTC().Format(time.RFC3339),
		},
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := true
	readyErr := ""
	if s.readyCheck != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.readyCheck(ctx); err != nil {
			ready = false
			readyErr = "dependency_unavailable"
			log.Printf("requestId=%s readiness check failed: %v", getRequestID(r.Context()), err)
		}
	}
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}
	writeJSON(w, statusCode, map[string]any{
		"code": "0",
		"data": map[string]any{
			"ready":     ready,
			"error":     readyErr,
			"requestId": getRequestID(r.Context()),
			"time":      s.nowFn().UTC().Format(time.RFC3339),
		},
	})
}

func isPositiveDecimal(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	n, err := strconv.ParseFloat(v, 64)
	return err == nil && n > 0
}

func isNonNegativeDecimal(v string) bool {
	if strings.TrimSpace(v) == "" {
		return false
	}
	n, err := strconv.ParseFloat(v, 64)
	return err == nil && n >= 0
}

func (s *Server) validateSignatureTimestamp(raw string) bool {
	if raw == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	now := s.nowFn().UTC()
	if t.After(now.Add(s.signatureMaxSkew)) {
		return false
	}
	return now.Sub(t) <= s.signatureMaxSkew
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

func maskDID(did string) string {
	if len(did) <= 8 {
		return "***"
	}
	return did[:6] + "***" + did[len(did)-2:]
}

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if reqID == "" {
			reqID = "req_" + strconv.FormatInt(s.nowFn().UnixNano(), 10)
		}
		w.Header().Set("X-Request-Id", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func m6FundsEnabled() bool {
	v := strings.TrimSpace(os.Getenv("FEATURE_M6_FUNDS"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *Server) withM6Funds(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m6FundsEnabled() {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-011", "message": "feature disabled"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func m7CardRiskEnabled() bool {
	v := strings.TrimSpace(os.Getenv("FEATURE_M7_CARD_RISK"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *Server) withM7CardRisk(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m7CardRiskEnabled() {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-012", "message": "feature disabled"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func m8SelfHostedEnabled() bool {
	v := strings.TrimSpace(os.Getenv("FEATURE_M8_SELF_HOSTED"))
	return v == "1" || strings.EqualFold(v, "true")
}

func (s *Server) withM8SelfHosted(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m8SelfHostedEnabled() {
			writeJSON(w, http.StatusNotFound, map[string]any{"code": "PAY-013", "message": "feature disabled"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isBase64Ed25519PubKey(raw string) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	return err == nil && len(decoded) == ed25519.PublicKeySize
}

func buildPaySignaturePayload(payerDID, merchantID, amount, idemKey, ts string) []byte {
	return []byte(payerDID + "|" + merchantID + "|" + amount + "|" + idemKey + "|" + ts)
}

func buildUpdateKeyProofPayload(agentDID, newPubKey, ts string) string {
	return strings.TrimSpace(agentDID) + "|" + strings.TrimSpace(newPubKey) + "|" + strings.TrimSpace(ts)
}

func verifyDIDSignature(pubKeyBase64, signatureBase64 string, payload []byte) bool {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pubKeyBase64))
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureBase64))
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubKeyBytes), payload, sigBytes)
}

func (s *Server) withAdminAuth(next http.Handler) http.Handler {
	return s.withRoleAuth(false, next)
}

func (s *Server) withReadAuth(next http.Handler) http.Handler {
	return s.withRoleAuth(true, next)
}

func (s *Server) withRoleAuth(allowReadonly bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminConfigured := strings.TrimSpace(s.adminBearerToken) != ""
		readonlyConfigured := strings.TrimSpace(s.readonlyToken) != ""
		if !adminConfigured && !readonlyConfigured {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if adminConfigured && secureEqual(token, s.adminBearerToken) {
			next.ServeHTTP(w, r)
			return
		}
		if allowReadonly && readonlyConfigured && secureEqual(token, s.readonlyToken) {
			next.ServeHTTP(w, r)
			return
		}
		log.Printf("requestId=%s unauthorized path=%s ip=%s readonlyAllowed=%t", getRequestID(r.Context()), r.URL.Path, clientIP(r), allowReadonly)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "unauthorized"})
	})
}

func (s *Server) withCallbackToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(s.callbackToken) == "" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSpace(r.Header.Get("X-Callback-Token"))
		if !secureEqual(token, s.callbackToken) {
			log.Printf("requestId=%s unauthorized callback path=%s ip=%s", getRequestID(r.Context()), r.URL.Path, clientIP(r))
			writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "unauthorized callback"})
			return
		}
		tsRaw := strings.TrimSpace(r.Header.Get("X-Callback-Timestamp"))
		if !s.validateSignatureTimestamp(tsRaw) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid callback timestamp"})
			return
		}
		idemKey := strings.TrimSpace(r.Header.Get("X-Callback-Idempotency-Key"))
		if idemKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing callback idempotency key"})
			return
		}
		nonce := strings.TrimSpace(r.Header.Get("X-Callback-Nonce"))
		if nonce == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "missing callback nonce"})
			return
		}
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request body"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
		digest := callbackDigest(rawBody, tsRaw, nonce)
		if existed, same := s.callbackDedupe.Seen(idemKey, digest, s.nowFn().UTC()); existed {
			if same {
				writeJSON(w, http.StatusOK, map[string]any{"code": "0", "message": "ok"})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]any{"code": "PAY-010", "message": "callback idempotency conflict"})
			return
		}
		if !s.callbackNonceLRU.allow("callback:"+nonce, s.nowFn().UTC()) {
			writeJSON(w, http.StatusConflict, map[string]any{"code": "PAY-010", "message": "replayed callback"})
			return
		}
		if strings.TrimSpace(s.callbackSignKey) != "" {
			version := strings.TrimSpace(r.Header.Get("X-Callback-Signature-Version"))
			if version != "v1" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "unsupported callback signature version"})
				return
			}
			var req statusCallbackReq
			if err := json.Unmarshal(rawBody, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": "PAY-010", "message": "invalid request"})
				return
			}
			expected := buildCallbackSignatureV1(s.callbackSignKey, req.TransactionID, req.Status, tsRaw, nonce, idemKey)
			got := strings.TrimSpace(r.Header.Get("X-Callback-Signature"))
			if !secureEqual(got, expected) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"code": "PAY-010", "message": "invalid callback signature"})
				return
			}
		}
		sw := &statusCaptureWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(sw, r)
		if sw.statusCode >= 200 && sw.statusCode < 300 {
			s.callbackDedupe.Mark(idemKey, digest, s.nowFn().UTC())
		}
	})
}

func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) appendAuditLog(r *http.Request, action string, resource string, detail map[string]any) {
	role := "admin"
	actor := "system"
	if token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); token != "" {
		if s.readonlyToken != "" && secureEqual(token, s.readonlyToken) {
			role = "readonly"
		}
		if len(token) > 8 {
			actor = token[:4] + "***" + token[len(token)-2:]
		} else {
			actor = "***"
		}
	}
	if err := s.svc.AppendAuditLog(actor, role, action, resource, getRequestID(r.Context()), detail); err != nil {
		log.Printf("requestId=%s append audit failed: %v", getRequestID(r.Context()), err)
	}
}

func buildCallbackSignatureV1(secret, transactionID, status, ts, nonce, idemKey string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	payload := strings.TrimSpace(transactionID) + "|" + strings.ToUpper(strings.TrimSpace(status)) + "|" + strings.TrimSpace(ts) + "|" + strings.TrimSpace(nonce) + "|" + strings.TrimSpace(idemKey)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

type statusCaptureWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

type callbackDedupeStore struct {
	mu      sync.Mutex
	window  time.Duration
	records map[string]callbackRecord
}

type callbackRecord struct {
	digest string
	seenAt time.Time
}

func newCallbackDedupeStore(window time.Duration) *callbackDedupeStore {
	return &callbackDedupeStore{
		window:  window,
		records: map[string]callbackRecord{},
	}
}

func (s *callbackDedupeStore) Seen(idemKey, digest string, now time.Time) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(now)
	record, ok := s.records[idemKey]
	if !ok {
		return false, false
	}
	return true, secureEqual(record.digest, digest)
}

func (s *callbackDedupeStore) Mark(idemKey, digest string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(now)
	s.records[idemKey] = callbackRecord{digest: digest, seenAt: now}
}

func (s *callbackDedupeStore) prune(now time.Time) {
	expireBefore := now.Add(-s.window)
	for key, record := range s.records {
		if record.seenAt.Before(expireBefore) {
			delete(s.records, key)
		}
	}
}

func callbackDigest(body []byte, ts, nonce string) string {
	sum := sha256.Sum256([]byte(string(body) + "|" + strings.TrimSpace(ts) + "|" + strings.TrimSpace(nonce)))
	return hex.EncodeToString(sum[:])
}
