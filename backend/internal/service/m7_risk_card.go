package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func shortSandboxHash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:8])
}

func (s *Service) ApplyVirtualCard(agentDID string, vaAccountID string, requestedLimit string, idemKey string) (VirtualCardRecord, error) {
	v, err := parseAmount(requestedLimit)
	if err != nil || v <= 0 {
		return VirtualCardRecord{}, &APIError{Code: "PAY-010", Message: "invalid credit limit"}
	}
	if strings.TrimSpace(idemKey) == "" {
		return VirtualCardRecord{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.actionIdem["vc_apply:"+idemKey]; ok {
		for _, c := range s.virtualCards {
			if c.SandboxReference == "idem:"+idemKey {
				return c, nil
			}
		}
		return VirtualCardRecord{}, &APIError{Code: "PAY-010", Message: "idempotent replay without card"}
	}
	acc, ok := s.accounts[agentDID]
	if !ok || acc.VAAccountID != strings.TrimSpace(vaAccountID) {
		return VirtualCardRecord{}, &APIError{Code: "PAY-010", Message: "account not found"}
	}
	cid := fmt.Sprintf("vc_%d", time.Now().UnixNano())
	masked := fmt.Sprintf("6888 **** **** %04d", time.Now().UnixNano()%10000)
	rec := VirtualCardRecord{
		CardID:           cid,
		AgentDID:         agentDID,
		VAAccountID:      acc.VAAccountID,
		MaskedPAN:        masked,
		Status:           "ACTIVE",
		CreditLimit:      v,
		SandboxReference: "idem:" + idemKey,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	s.virtualCards = append([]VirtualCardRecord{rec}, s.virtualCards...)
	s.actionIdem["vc_apply:"+idemKey] = struct{}{}
	s.appendRiskAuditLocked("payment.card.apply", agentDID, "", "", map[string]any{"cardId": cid, "creditLimit": v})
	return rec, nil
}

func (s *Service) PayVirtualCard(agentDID string, cardID string, merchantID string, amount string, idemKey string) (string, error) {
	if strings.TrimSpace(idemKey) == "" {
		return "", &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	amt, err := parseAmount(amount)
	if err != nil || amt <= 0 {
		return "", &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if tid, ok := s.cardPayIdem[idemKey]; ok {
		return tid, nil
	}
	var idx int = -1
	for i := range s.virtualCards {
		if s.virtualCards[i].CardID == cardID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", &APIError{Code: "PAY-010", Message: "card not found"}
	}
	card := &s.virtualCards[idx]
	if strings.TrimSpace(agentDID) != card.AgentDID {
		return "", &APIError{Code: "PAY-010", Message: "card not found"}
	}
	if strings.ToUpper(card.Status) != "ACTIVE" {
		return "", &APIError{Code: "PAY-002", Message: "card not active"}
	}
	if amt > card.CreditLimit {
		return "", &APIError{Code: "PAY-002", Message: "over card limit"}
	}
	acc := s.accounts[card.AgentDID]
	if acc == nil {
		return "", &APIError{Code: "PAY-010", Message: "account not found"}
	}
	if acc.Balance < amt {
		return "", &APIError{Code: "PAY-003", Message: "agent va insufficient balance"}
	}
	if r := s.evaluateP2Risk(merchantID, "GUSD", amt); r != nil {
		s.appendRiskAuditLocked("risk.transaction.check", card.AgentDID, merchantID, "", map[string]any{"amount": amt, "decision": "BLOCKED", "reason": r.Message})
		return "", r
	}
	acc.Balance -= amt
	txID := fmt.Sprintf("cardpay_%d", time.Now().UnixNano())
	s.cardPayIdem[idemKey] = txID
	s.appendRiskAuditLocked("payment.card.pay", card.AgentDID, merchantID, txID, map[string]any{"amount": amt, "cardId": cardID})
	return txID, nil
}

func (s *Service) ManageVirtualCard(cardID string, operation string, adjustAmount string) (VirtualCardRecord, error) {
	op := strings.ToUpper(strings.TrimSpace(operation))
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.virtualCards {
		if s.virtualCards[i].CardID != cardID {
			continue
		}
		switch op {
		case "FREEZE":
			s.virtualCards[i].Status = "FROZEN"
		case "UNFREEZE", "ACTIVATE":
			s.virtualCards[i].Status = "ACTIVE"
		case "ADJUST_LIMIT":
			v, err := parseAmount(adjustAmount)
			if err != nil || v <= 0 {
				return VirtualCardRecord{}, &APIError{Code: "PAY-010", Message: "invalid limit"}
			}
			s.virtualCards[i].CreditLimit = v
		default:
			return VirtualCardRecord{}, &APIError{Code: "PAY-010", Message: "invalid operation"}
		}
		s.virtualCards[i].UpdatedAt = time.Now().UTC()
		s.appendRiskAuditLocked("payment.card.manage", s.virtualCards[i].AgentDID, "", "", map[string]any{"cardId": cardID, "operation": op})
		return s.virtualCards[i], nil
	}
	return VirtualCardRecord{}, &APIError{Code: "PAY-010", Message: "card not found"}
}

func (s *Service) RiskTransactionCheck(agentDID string, merchantID string, amount string, transactionID string) (map[string]any, error) {
	amt, err := parseAmount(amount)
	if err != nil || amt <= 0 {
		return nil, &APIError{Code: "PAY-010", Message: "invalid amount"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r := s.evaluateP2Risk(merchantID, "GUSD", amt); r != nil {
		s.appendRiskAuditLocked("risk.transaction.check", agentDID, merchantID, transactionID, map[string]any{"amount": amt, "decision": "BLOCKED", "code": r.Code})
		return map[string]any{
			"decision":      "BLOCK",
			"code":          r.Code,
			"message":       r.Message,
			"agentDid":      agentDID,
			"merchantId":    merchantID,
			"amount":        amt,
			"transactionId": transactionID,
		}, nil
	}
	s.appendRiskAuditLocked("risk.transaction.check", agentDID, merchantID, transactionID, map[string]any{"amount": amt, "decision": "ALLOW"})
	return map[string]any{
		"decision":      "ALLOW",
		"agentDid":      agentDID,
		"merchantId":    merchantID,
		"amount":        amt,
		"transactionId": transactionID,
	}, nil
}

func (s *Service) RiskKYCVerify(agentDID string, documentReference string, idemKey string) (PartyKYCStatus, error) {
	if strings.TrimSpace(idemKey) == "" {
		return PartyKYCStatus{}, &APIError{Code: "PAY-008", Message: "idempotency key required"}
	}
	agentDID = strings.TrimSpace(agentDID)
	if agentDID == "" {
		return PartyKYCStatus{}, &APIError{Code: "PAY-010", Message: "invalid request"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[agentDID]; !ok {
		return PartyKYCStatus{}, &APIError{Code: "PAY-010", Message: "agent not found"}
	}
	if _, ok := s.actionIdem["kyc:"+idemKey]; ok {
		return s.kycByAgent[agentDID], nil
	}
	ext := shortSandboxHash(agentDID, documentReference, idemKey)
	st := PartyKYCStatus{
		AgentDID:          agentDID,
		Status:            "APPROVED",
		Tier:              "SANDBOX_T1",
		ExternalReference: "sandbox_kyc_" + ext,
		VerifiedAt:        time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	s.kycByAgent[agentDID] = st
	s.actionIdem["kyc:"+idemKey] = struct{}{}
	s.appendRiskAuditLocked("risk.kyc.verify", agentDID, "", "", map[string]any{"documentReference": strings.TrimSpace(documentReference), "status": st.Status})
	return st, nil
}

func (s *Service) RiskAuditQuery(agentDID string, merchantID string, limit int, offset int) []RiskAuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	out := make([]RiskAuditEntry, 0, limit)
	skipped := 0
	for _, e := range s.riskAuditEntries {
		if agentDID != "" && e.AgentDID != agentDID {
			continue
		}
		if merchantID != "" && e.MerchantID != merchantID {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Service) appendRiskAuditLocked(category string, agentDID string, merchantID string, transactionID string, detail map[string]any) {
	raw, _ := json.Marshal(detail)
	s.riskAuditSeq++
	ent := RiskAuditEntry{
		ID:            s.riskAuditSeq,
		Category:      category,
		AgentDID:      agentDID,
		MerchantID:    merchantID,
		TransactionID: transactionID,
		Detail:        raw,
		CreatedAt:     time.Now().UTC(),
	}
	s.riskAuditEntries = append([]RiskAuditEntry{ent}, s.riskAuditEntries...)
}
