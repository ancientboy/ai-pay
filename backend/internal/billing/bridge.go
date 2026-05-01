package billing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const bridgeAPIBase = "https://api.bridge.xyz/v0"

type BridgeCountry struct {
	Name   string `json:"name"`
	Alpha3 string `json:"alpha3"`
}

type BridgeKYCLinkResult struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	KYCLink    string `json:"kycLink"`
	TOSLink    string `json:"tosLink"`
	KYCStatus  string `json:"kycStatus"`
	TOSStatus  string `json:"tosStatus"`
}

type BridgeVirtualAccountResult struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	Customer  string         `json:"customerId"`
	CreatedAt string         `json:"createdAt"`
	Source    map[string]any `json:"sourceDepositInstructions"`
}

func bridgeAPIKey() string {
	return strings.TrimSpace(os.Getenv("BRIDGE_API_KEY"))
}

func BridgeConfigured() bool {
	return bridgeAPIKey() != ""
}

func bridgeRequest(method, path string, body any, idempotencyKey string) ([]byte, error) {
	apiKey := bridgeAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("BRIDGE_API_KEY not configured")
	}
	fullURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BRIDGE_API_BASE_URL")), "/")
	if fullURL == "" {
		fullURL = bridgeAPIBase
	}
	fullURL += path
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, fullURL, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", strings.TrimSpace(idempotencyKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bridge request failed: %s: %s", resp.Status, truncate(string(raw), 500))
	}
	return raw, nil
}

func GetBridgeCountries() ([]BridgeCountry, error) {
	raw, err := bridgeRequest(http.MethodGet, "/lists/countries", nil, "")
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Data []struct {
			Name   string `json:"name"`
			Alpha3 string `json:"alpha3"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]BridgeCountry, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if strings.TrimSpace(item.Alpha3) == "" {
			continue
		}
		out = append(out, BridgeCountry{Name: item.Name, Alpha3: strings.ToUpper(item.Alpha3)})
	}
	return out, nil
}

func CreateBridgeKYCLink(fullName, email, kycType, redirectURI string, endorsements []string, idempotencyKey string) (BridgeKYCLinkResult, error) {
	body := map[string]any{
		"full_name": strings.TrimSpace(fullName),
		"email":     strings.TrimSpace(email),
		"type":      strings.TrimSpace(strings.ToLower(kycType)),
	}
	if strings.TrimSpace(redirectURI) != "" {
		body["redirect_uri"] = strings.TrimSpace(redirectURI)
	}
	if len(endorsements) > 0 {
		body["endorsements"] = endorsements
	}
	raw, err := bridgeRequest(http.MethodPost, "/kyc_links", body, idempotencyKey)
	if err != nil {
		return BridgeKYCLinkResult{}, err
	}
	var parsed struct {
		ID         string `json:"id"`
		CustomerID string `json:"customer_id"`
		KYCLink    string `json:"kyc_link"`
		TOSLink    string `json:"tos_link"`
		KYCStatus  string `json:"kyc_status"`
		TOSStatus  string `json:"tos_status"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return BridgeKYCLinkResult{}, err
	}
	return BridgeKYCLinkResult{
		ID:         parsed.ID,
		CustomerID: parsed.CustomerID,
		KYCLink:    parsed.KYCLink,
		TOSLink:    parsed.TOSLink,
		KYCStatus:  parsed.KYCStatus,
		TOSStatus:  parsed.TOSStatus,
	}, nil
}

func CreateBridgeVirtualAccount(customerID string, sourceCurrency string, destinationCurrency string, paymentRail string, address string, developerFeePercent string, idempotencyKey string) (BridgeVirtualAccountResult, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return BridgeVirtualAccountResult{}, fmt.Errorf("customer id required")
	}
	body := map[string]any{
		"source": map[string]any{
			"currency": strings.TrimSpace(strings.ToLower(sourceCurrency)),
		},
		"destination": map[string]any{
			"currency":     strings.TrimSpace(strings.ToLower(destinationCurrency)),
			"payment_rail": strings.TrimSpace(strings.ToLower(paymentRail)),
			"address":      strings.TrimSpace(address),
		},
	}
	if strings.TrimSpace(developerFeePercent) != "" {
		body["developer_fee_percent"] = strings.TrimSpace(developerFeePercent)
	}
	raw, err := bridgeRequest(http.MethodPost, "/customers/"+url.PathEscape(customerID)+"/virtual_accounts", body, idempotencyKey)
	if err != nil {
		return BridgeVirtualAccountResult{}, err
	}
	var parsed struct {
		ID                        string         `json:"id"`
		Status                    string         `json:"status"`
		CustomerID                string         `json:"customer_id"`
		CreatedAt                 string         `json:"created_at"`
		SourceDepositInstructions map[string]any `json:"source_deposit_instructions"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return BridgeVirtualAccountResult{}, err
	}
	return BridgeVirtualAccountResult{
		ID:        parsed.ID,
		Status:    parsed.Status,
		Customer:  parsed.CustomerID,
		CreatedAt: parsed.CreatedAt,
		Source:    parsed.SourceDepositInstructions,
	}, nil
}

func BridgeVACountriesFromRecognized(items []BridgeCountry) []map[string]any {
	nameByCode := map[string]string{}
	for _, item := range items {
		nameByCode[item.Alpha3] = item.Name
	}
	getName := func(code, fallback string) string {
		if v := strings.TrimSpace(nameByCode[code]); v != "" {
			return v
		}
		return fallback
	}
	return []map[string]any{
		{"alpha3": "USA", "name": getName("USA", "United States"), "sourceCurrency": "usd", "rails": []string{"ach_push", "wire"}},
		{"alpha3": "MEX", "name": getName("MEX", "Mexico"), "sourceCurrency": "mxn", "rails": []string{"spei"}},
		{"alpha3": "BRA", "name": getName("BRA", "Brazil"), "sourceCurrency": "brl", "rails": []string{"pix"}},
		{"alpha3": "GBR", "name": getName("GBR", "United Kingdom"), "sourceCurrency": "gbp", "rails": []string{"faster_payments"}},
		{"alpha3": "COL", "name": getName("COL", "Colombia"), "sourceCurrency": "cop", "rails": []string{"bre_b"}},
		{"alpha3": "EEA", "name": "EEA / SEPA region", "sourceCurrency": "eur", "rails": []string{"sepa"}},
	}
}

func MockBridgeKYCLink(fullName, email, kycType string) BridgeKYCLinkResult {
	token := fmt.Sprintf("mock_%d", time.Now().UnixNano())
	return BridgeKYCLinkResult{
		ID:         "kyc_link_" + token,
		CustomerID: "customer_" + token,
		KYCLink:    "https://bridge.mock.local/kyc/" + token + "?email=" + url.QueryEscape(strings.TrimSpace(email)),
		TOSLink:    "https://bridge.mock.local/tos/" + token,
		KYCStatus:  "not_started",
		TOSStatus:  "pending",
	}
}
