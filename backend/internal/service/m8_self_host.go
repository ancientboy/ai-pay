package service

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type WalletBinding struct {
	AgentDID      string    `json:"agentDid"`
	WalletAddress string    `json:"walletAddress"`
	Label         string    `json:"label,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// AuthSession is a time-bounded authorization scope for payment signing (M8).
type AuthSession struct {
	SessionID string    `json:"sessionId"`
	AgentDID  string    `json:"agentDid"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// PaymentSignRequestRecord is a pending payment to be confirmed via Submit (M8).
type PaymentSignRequestRecord struct {
	SignID     string    `json:"signId"`
	AgentDID   string    `json:"agentDid"`
	MerchantID string    `json:"merchantId"`
	Amount     string    `json:"amount"`
	SessionID  string    `json:"sessionId,omitempty"`
	Status     string    `json:"status"`
	ExpiresAt  time.Time `json:"expiresAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

type m8session struct {
	SessionID string
	AgentDID  string
	Status    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type m8signReq struct {
	SignID     string
	AgentDID   string
	MerchantID string
	Amount     string
	SessionID  string
	Status     string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

func (s *Service) BindWallet(agentDID string, walletAddress string, label string) error {
	agentDID = strings.TrimSpace(agentDID)
	walletAddress = strings.TrimSpace(walletAddress)
	if agentDID == "" || walletAddress == "" {
		return &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[agentDID]; !ok {
		return &APIError{Code: "PAY-010", Message: "agent not found"}
	}
	s.walletBindings[agentDID] = WalletBinding{
		AgentDID:      agentDID,
		WalletAddress: walletAddress,
		Label:         strings.TrimSpace(label),
		UpdatedAt:     time.Now().UTC(),
	}
	return nil
}

func (s *Service) UnbindWallet(agentDID string) error {
	agentDID = strings.TrimSpace(agentDID)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.walletBindings, agentDID)
	return nil
}

func (s *Service) GetWalletBinding(agentDID string) (WalletBinding, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.walletBindings[strings.TrimSpace(agentDID)]
	return w, ok
}

func (s *Service) CreateAuthSession(agentDID string, ttlMinutes int, idemKey string) (AuthSession, error) {
	if strings.TrimSpace(idemKey) == "" {
		return AuthSession{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	if ttlMinutes <= 0 {
		ttlMinutes = 60
	}
	if ttlMinutes > 1440 {
		ttlMinutes = 1440
	}
	agentDID = strings.TrimSpace(agentDID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[agentDID]; !ok {
		return AuthSession{}, &APIError{Code: "PAY-010", Message: "agent not found"}
	}
	if sid, ok := s.m8sessCreateIdem[idemKey]; ok {
		if sess, ok := s.m8sessions[sid]; ok {
			return AuthSession{
				SessionID: sess.SessionID,
				AgentDID:  sess.AgentDID,
				Status:    sess.Status,
				ExpiresAt: sess.ExpiresAt,
				CreatedAt: sess.CreatedAt,
			}, nil
		}
	}
	sid := fmt.Sprintf("sess_%d", time.Now().UnixNano())
	now := time.Now().UTC()
	exp := now.Add(time.Duration(ttlMinutes) * time.Minute)
	rec := m8session{
		SessionID: sid,
		AgentDID:  agentDID,
		Status:    "ACTIVE",
		ExpiresAt: exp,
		CreatedAt: now,
	}
	s.m8sessions[sid] = rec
	s.m8sessCreateIdem[idemKey] = sid
	return AuthSession{
		SessionID: rec.SessionID,
		AgentDID:  rec.AgentDID,
		Status:    rec.Status,
		ExpiresAt: rec.ExpiresAt,
		CreatedAt: rec.CreatedAt,
	}, nil
}

func (s *Service) RevokeAuthSession(agentDID string, sessionID string, idemKey string) error {
	if strings.TrimSpace(idemKey) == "" {
		return &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := "sess_revoke:" + idemKey
	if _, ok := s.actionIdem[key]; ok {
		return nil
	}
	sess, ok := s.m8sessions[strings.TrimSpace(sessionID)]
	if !ok || sess.AgentDID != strings.TrimSpace(agentDID) {
		return &APIError{Code: "PAY-010", Message: "session not found"}
	}
	sess.Status = "REVOKED"
	s.m8sessions[strings.TrimSpace(sessionID)] = sess
	s.actionIdem[key] = struct{}{}
	return nil
}

func (s *Service) authSessionActive(agentDID string, sessionID string, now time.Time) bool {
	if strings.TrimSpace(sessionID) == "" {
		return true
	}
	sess, ok := s.m8sessions[strings.TrimSpace(sessionID)]
	if !ok || sess.AgentDID != strings.TrimSpace(agentDID) {
		return false
	}
	if strings.ToUpper(sess.Status) != "ACTIVE" {
		return false
	}
	return now.Before(sess.ExpiresAt)
}

func (s *Service) RequestPaymentSign(agentDID string, merchantID string, amount string, sessionID string, idemKey string) (PaymentSignRequestRecord, error) {
	if strings.TrimSpace(idemKey) == "" {
		return PaymentSignRequestRecord{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	if _, err := parseAmount(amount); err != nil {
		return PaymentSignRequestRecord{}, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	agentDID = strings.TrimSpace(agentDID)
	merchantID = strings.TrimSpace(merchantID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[agentDID]; !ok {
		return PaymentSignRequestRecord{}, &APIError{Code: "PAY-010", Message: "agent not found"}
	}
	if sid := strings.TrimSpace(sessionID); sid != "" && !s.authSessionActive(agentDID, sid, time.Now().UTC()) {
		return PaymentSignRequestRecord{}, &APIError{Code: "PAY-002", Message: "session invalid or expired"}
	}
	if prevSign, ok := s.m8signReqIdem[idemKey]; ok {
		if r, ok := s.m8signReqs[prevSign]; ok {
			return PaymentSignRequestRecord{
				SignID:     r.SignID,
				AgentDID:   r.AgentDID,
				MerchantID: r.MerchantID,
				Amount:     r.Amount,
				SessionID:  r.SessionID,
				Status:     r.Status,
				ExpiresAt:  r.ExpiresAt,
				CreatedAt:  r.CreatedAt,
			}, nil
		}
	}
	signID := fmt.Sprintf("sig_%d", time.Now().UnixNano())
	now := time.Now().UTC()
	rec := m8signReq{
		SignID:     signID,
		AgentDID:   agentDID,
		MerchantID: merchantID,
		Amount:     strings.TrimSpace(amount),
		SessionID:  strings.TrimSpace(sessionID),
		Status:     "PENDING",
		ExpiresAt:  now.Add(15 * time.Minute),
		CreatedAt:  now,
	}
	s.m8signReqs[signID] = rec
	s.m8signReqIdem[idemKey] = signID
	return PaymentSignRequestRecord{
		SignID:     rec.SignID,
		AgentDID:   rec.AgentDID,
		MerchantID: rec.MerchantID,
		Amount:     rec.Amount,
		SessionID:  rec.SessionID,
		Status:     rec.Status,
		ExpiresAt:  rec.ExpiresAt,
		CreatedAt:  rec.CreatedAt,
	}, nil
}

func (s *Service) SubmitSignedPayment(signID string, req PayRequest) (PayResponse, *APIError) {
	s.mu.Lock()
	rec, ok := s.m8signReqs[strings.TrimSpace(signID)]
	if !ok {
		s.mu.Unlock()
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "sign request not found"}
	}
	if strings.ToUpper(rec.Status) != "PENDING" {
		s.mu.Unlock()
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "sign request not pending"}
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		rec.Status = "EXPIRED"
		s.m8signReqs[strings.TrimSpace(signID)] = rec
		s.mu.Unlock()
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "sign request expired"}
	}
	pAmt, err := parseAmount(strings.TrimSpace(req.Amount))
	recAmt, err2 := parseAmount(rec.Amount)
	if err != nil || err2 != nil || math.Abs(pAmt-recAmt) > 1e-9 {
		s.mu.Unlock()
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "sign request mismatch"}
	}
	if strings.TrimSpace(req.PayerDID) != rec.AgentDID ||
		strings.TrimSpace(req.MerchantID) != rec.MerchantID {
		s.mu.Unlock()
		return PayResponse{}, &APIError{Code: "PAY-010", Message: "sign request mismatch"}
	}
	if rec.SessionID != "" && !s.authSessionActive(rec.AgentDID, rec.SessionID, time.Now().UTC()) {
		s.mu.Unlock()
		return PayResponse{}, &APIError{Code: "PAY-002", Message: "session invalid or expired"}
	}
	s.mu.Unlock()

	resp, apiErr := s.Pay(req)

	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.m8signReqs[strings.TrimSpace(signID)]; ok && apiErr == nil {
		cur.Status = "COMPLETED"
		s.m8signReqs[strings.TrimSpace(signID)] = cur
	}
	return resp, apiErr
}
