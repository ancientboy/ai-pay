package service

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type channelDecision string

const (
	channelSettleNow channelDecision = "SETTLED"
	channelSettling  channelDecision = "SETTLING"
	channelFail      channelDecision = "FAILED"
)

func isValidChannelMode(mode string) bool {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "SETTLE", "ASYNC", "FAIL":
		return true
	default:
		return false
	}
}

func evaluateRiskWithConfig(cfg RiskConfig, merchantID string, amount float64) *APIError {
	if !cfg.Enabled {
		return nil
	}
	merchant := strings.TrimSpace(merchantID)
	for _, blocked := range cfg.BlockedMerchants {
		if blocked == merchant {
			return &APIError{Code: "PAY-002", Message: "risk blocked merchant"}
		}
	}
	if amount > cfg.SingleAmountLimit {
		return &APIError{Code: "PAY-002", Message: "risk amount exceeded"}
	}
	return nil
}

func decideChannelByRoute(routes map[string]ChannelRoute, merchantID string) channelDecision {
	merchant := strings.TrimSpace(merchantID)
	if item, ok := routes[merchant]; ok {
		switch strings.ToUpper(strings.TrimSpace(item.Mode)) {
		case "FAIL":
			return channelFail
		case "ASYNC":
			return channelSettling
		default:
			return channelSettleNow
		}
	}
	switch {
	case strings.HasPrefix(merchant, "m_fail"):
		return channelFail
	case strings.HasPrefix(merchant, "m_async"):
		return channelSettling
	default:
		return channelSettleNow
	}
}

func (s *Service) evaluateP2Risk(merchantID string, amount float64) *APIError {
	cfg := s.riskConfig
	cfg.BlockedMerchants = append([]string{}, s.riskConfig.BlockedMerchants...)
	return evaluateRiskWithConfig(cfg, merchantID, amount)
}

func (s *Service) decideChannelPath(merchantID string) channelDecision {
	return decideChannelByRoute(s.channelRoutes, merchantID)
}

func (s *PersistentService) getRiskConfig() (RiskConfig, error) {
	cfg := RiskConfig{
		Enabled:           true,
		SingleAmountLimit: 1000,
		BlockedMerchants:  []string{"m_risk_block"},
		UpdatedAt:         time.Now().UTC(),
	}
	var blockedRaw string
	err := s.store.DB.QueryRow(`SELECT enabled, single_amount_limit, blocked_merchants, updated_at FROM risk_config WHERE id = 1`).
		Scan(&cfg.Enabled, &cfg.SingleAmountLimit, &blockedRaw, &cfg.UpdatedAt)
	if err == sql.ErrNoRows {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(blockedRaw) != "" {
		_ = json.Unmarshal([]byte(blockedRaw), &cfg.BlockedMerchants)
	}
	return cfg, nil
}

func (s *PersistentService) loadChannelRoutes() map[string]ChannelRoute {
	rows, err := s.store.DB.Query(`SELECT merchant_id, mode, updated_at FROM channel_route`)
	if err != nil {
		return map[string]ChannelRoute{}
	}
	defer rows.Close()
	out := map[string]ChannelRoute{}
	for rows.Next() {
		var item ChannelRoute
		if err := rows.Scan(&item.MerchantID, &item.Mode, &item.UpdatedAt); err != nil {
			continue
		}
		out[item.MerchantID] = item
	}
	return out
}

func (s *PersistentService) evaluateP2Risk(merchantID string, amount float64) *APIError {
	cfg, err := s.getRiskConfig()
	if err != nil {
		return &APIError{Code: "PAY-010", Message: "load risk config failed"}
	}
	return evaluateRiskWithConfig(cfg, merchantID, amount)
}

func (s *PersistentService) decideChannelPath(merchantID string) channelDecision {
	return decideChannelByRoute(s.loadChannelRoutes(), merchantID)
}
