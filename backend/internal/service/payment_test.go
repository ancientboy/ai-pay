package service

import (
	"encoding/base64"
	"strconv"
	"sync"
	"testing"
)

func TestPaySuccess(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:a1")
	acc := svc.CreateAccount("did:gusd:agent:a1")
	svc.Recharge(acc.VAAccountID, "100", "rch-1")
	svc.SetAuthorizeRule("did:gusd:agent:a1", "50", "200", []string{"m1"})

	resp, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:a1",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-1",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "SETTLED" {
		t.Fatalf("expected SETTLED got %s", resp.Status)
	}
	tx, err2 := svc.QueryStatus(resp.TransactionID)
	if err2 != nil {
		t.Fatalf("query status err: %v", err2)
	}
	if tx.Fee <= 0 {
		t.Fatalf("expected fee > 0")
	}
	balance, err3 := svc.BalanceByVA(acc.VAAccountID)
	if err3 != nil {
		t.Fatalf("query balance err: %v", err3)
	}
	if balance != 90 {
		t.Fatalf("expected balance 90 got %v", balance)
	}
	if svc.accounts["did:gusd:agent:a1"].FrozenBalance != 0 {
		t.Fatalf("expected frozen balance to be zero after debit")
	}
}

func TestPayInsufficientBalance(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:a2")
	_ = svc.CreateAccount("did:gusd:agent:a2")
	svc.SetAuthorizeRule("did:gusd:agent:a2", "50", "200", []string{"m1"})

	_, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:a2",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-2",
		Signature:      "sig",
	})
	if err == nil || err.Code != "PAY-003" {
		t.Fatalf("expected PAY-003 got %+v", err)
	}
}

func TestPayIdempotency(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:a3")
	acc := svc.CreateAccount("did:gusd:agent:a3")
	svc.Recharge(acc.VAAccountID, "100", "rch-2")
	svc.SetAuthorizeRule("did:gusd:agent:a3", "50", "200", []string{"m1"})

	first, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:a3",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-3",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:a3",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-3",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.TransactionID != second.TransactionID {
		t.Fatalf("idempotency broken")
	}
}

func TestPayDailyLimitExceeded(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:a4")
	acc := svc.CreateAccount("did:gusd:agent:a4")
	svc.Recharge(acc.VAAccountID, "100", "rch-3")
	svc.SetAuthorizeRule("did:gusd:agent:a4", "50", "15", []string{"m1"})

	_, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:a4",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-4-1",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	_, err = svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:a4",
		MerchantID:     "m1",
		Amount:         "10",
		IdempotencyKey: "idem-4-2",
		Signature:      "sig",
	})
	if err == nil || err.Code != "PAY-002" {
		t.Fatalf("expected PAY-002 daily limit exceeded, got %+v", err)
	}
}

func TestPayChannelTimeoutRollsBackFrozenBalance(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:rollback")
	acc := svc.CreateAccount("did:gusd:agent:rollback")
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-4")
	_ = svc.SetAuthorizeRule("did:gusd:agent:rollback", "100", "500", []string{"m_fail"})

	_, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:rollback",
		MerchantID:     "m_fail",
		Amount:         "10",
		IdempotencyKey: "idem-rollback-1",
		Signature:      "sig",
	})
	if err == nil || err.Code != "PAY-007" {
		t.Fatalf("expected PAY-007 got %+v", err)
	}
	balance, queryErr := svc.BalanceByVA(acc.VAAccountID)
	if queryErr != nil {
		t.Fatalf("query balance failed: %v", queryErr)
	}
	if balance != 100 {
		t.Fatalf("expected rollback balance 100 got %v", balance)
	}
	if svc.accounts["did:gusd:agent:rollback"].FrozenBalance != 0 {
		t.Fatalf("expected frozen balance to be zero after rollback")
	}
}

func TestPayAsyncCreatesSettlingWithFrozenBalance(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:async")
	acc := svc.CreateAccount("did:gusd:agent:async")
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-async-1")
	_ = svc.SetAuthorizeRule("did:gusd:agent:async", "100", "500", []string{"m_async"})

	resp, err := svc.Pay(PayRequest{
		PayerDID:       "did:gusd:agent:async",
		MerchantID:     "m_async",
		Amount:         "10",
		IdempotencyKey: "idem-async-1",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != "SETTLING" {
		t.Fatalf("expected SETTLING got %s", resp.Status)
	}
	balance, qErr := svc.BalanceByVA(acc.VAAccountID)
	if qErr != nil {
		t.Fatalf("balance query failed: %v", qErr)
	}
	if balance != 90 {
		t.Fatalf("expected available balance 90 got %v", balance)
	}
	if svc.accounts["did:gusd:agent:async"].FrozenBalance != 10 {
		t.Fatalf("expected frozen balance 10 got %v", svc.accounts["did:gusd:agent:async"].FrozenBalance)
	}
}

func TestResolveSettlingToSettled(t *testing.T) {
	svc := New()
	agent := "did:gusd:agent:resolve-ok"
	_ = svc.RegisterAgent(agent)
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-resolve-ok-1")
	_ = svc.SetAuthorizeRule(agent, "100", "500", []string{"m_async"})

	resp, err := svc.Pay(PayRequest{
		PayerDID:       agent,
		MerchantID:     "m_async",
		Amount:         "10",
		IdempotencyKey: "idem-resolve-ok-1",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("pay async failed: %v", err)
	}
	if err := svc.ResolveSettling(resp.TransactionID, true); err != nil {
		t.Fatalf("resolve settling failed: %v", err)
	}
	tx, qErr := svc.QueryStatus(resp.TransactionID)
	if qErr != nil {
		t.Fatalf("query status failed: %v", qErr)
	}
	if tx.Status != "SETTLED" {
		t.Fatalf("expected settled got %s", tx.Status)
	}
	if svc.accounts[agent].FrozenBalance != 0 {
		t.Fatalf("expected frozen balance 0 got %v", svc.accounts[agent].FrozenBalance)
	}
}

func TestUnfreezeSettlingTransaction(t *testing.T) {
	svc := New()
	agent := "did:gusd:agent:unfreeze"
	_ = svc.RegisterAgent(agent)
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-unfreeze-1")
	_ = svc.SetAuthorizeRule(agent, "100", "500", []string{"m_async"})
	resp, err := svc.Pay(PayRequest{
		PayerDID:       agent,
		MerchantID:     "m_async",
		Amount:         "20",
		IdempotencyKey: "idem-unfreeze-1",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("pay failed: %v", err)
	}
	if err := svc.Unfreeze(resp.TransactionID, "unfreeze-idem-1"); err != nil {
		t.Fatalf("unfreeze failed: %v", err)
	}
	balance, _ := svc.BalanceByVA(acc.VAAccountID)
	if balance != 100 {
		t.Fatalf("expected balance restored to 100 got %v", balance)
	}
}

func TestRefundSettledTransaction(t *testing.T) {
	svc := New()
	agent := "did:gusd:agent:refund"
	_ = svc.RegisterAgent(agent)
	acc := svc.CreateAccount(agent)
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-refund-1")
	_ = svc.SetAuthorizeRule(agent, "100", "500", []string{"m1"})
	resp, err := svc.Pay(PayRequest{
		PayerDID:       agent,
		MerchantID:     "m1",
		Amount:         "20",
		IdempotencyKey: "idem-refund-pay-1",
		Signature:      "sig",
	})
	if err != nil {
		t.Fatalf("pay failed: %v", err)
	}
	if err := svc.Refund(resp.TransactionID, "refund-idem-1"); err != nil {
		t.Fatalf("refund failed: %v", err)
	}
	balance, _ := svc.BalanceByVA(acc.VAAccountID)
	if balance != 100 {
		t.Fatalf("expected refunded balance 100 got %v", balance)
	}
}

func TestPayConcurrentNoOverdraft(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:concurrent")
	acc := svc.CreateAccount("did:gusd:agent:concurrent")
	_ = svc.Recharge(acc.VAAccountID, "100", "rch-concurrent-1")
	_ = svc.SetAuthorizeRule("did:gusd:agent:concurrent", "100", "500", []string{"m1"})

	var wg sync.WaitGroup
	results := make(chan *APIError, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := svc.Pay(PayRequest{
				PayerDID:       "did:gusd:agent:concurrent",
				MerchantID:     "m1",
				Amount:         "80",
				IdempotencyKey: "idem-concurrent-" + strconv.Itoa(idx),
				Signature:      "sig",
			})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	success := 0
	failedInsufficient := 0
	for err := range results {
		if err == nil {
			success++
			continue
		}
		if err.Code == "PAY-003" {
			failedInsufficient++
		}
	}
	if success != 1 || failedInsufficient != 1 {
		t.Fatalf("expected one success and one insufficient failure, got success=%d insufficient=%d", success, failedInsufficient)
	}
}

func TestCreateAccountIncludesVACardNo(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:card-1")
	acc := svc.CreateAccount("did:gusd:agent:card-1")
	if acc.VACardNo == "" {
		t.Fatalf("expected va card no to be generated")
	}
}

func TestAgentPublicKeyRoundTrip(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:key-1")
	pub := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	if err := svc.SetAgentPublicKey("did:gusd:agent:key-1", pub); err != nil {
		t.Fatalf("set pub key failed: %v", err)
	}
	got, err := svc.AgentPublicKey("did:gusd:agent:key-1")
	if err != nil {
		t.Fatalf("get pub key failed: %v", err)
	}
	if got != pub {
		t.Fatalf("expected %s got %s", pub, got)
	}
}

func TestRechargeByVACardNo(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:card-2")
	acc := svc.CreateAccount("did:gusd:agent:card-2")

	if err := svc.Recharge(acc.VACardNo, "12.5", "rch-5"); err != nil {
		t.Fatalf("recharge by card no failed: %v", err)
	}
	balance, err := svc.BalanceByVA(acc.VAAccountID)
	if err != nil {
		t.Fatalf("query balance failed: %v", err)
	}
	if balance != 12.5 {
		t.Fatalf("expected balance 12.5 got %v", balance)
	}
}

func BenchmarkPayInMemory(b *testing.B) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:bench")
	acc := svc.CreateAccount("did:gusd:agent:bench")
	_ = svc.Recharge(acc.VAAccountID, "1000000000", "rch-6")
	_ = svc.SetAuthorizeRule("did:gusd:agent:bench", "1000", "999999999", []string{"m1"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Pay(PayRequest{
			PayerDID:       "did:gusd:agent:bench",
			MerchantID:     "m1",
			Amount:         "1",
			IdempotencyKey: "idem-bench-" + strconv.Itoa(i+1),
			Signature:      "sig",
		})
		if err != nil {
			b.Fatalf("pay failed: %v", err)
		}
	}
}
