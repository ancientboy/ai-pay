package service

import (
	"strconv"
	"testing"
)

func TestPaySuccess(t *testing.T) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:a1")
	acc := svc.CreateAccount("did:gusd:agent:a1")
	svc.Recharge(acc.VAAccountID, "100")
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
	svc.Recharge(acc.VAAccountID, "100")
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
	svc.Recharge(acc.VAAccountID, "100")
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

func BenchmarkPayInMemory(b *testing.B) {
	svc := New()
	_ = svc.RegisterAgent("did:gusd:agent:bench")
	acc := svc.CreateAccount("did:gusd:agent:bench")
	_ = svc.Recharge(acc.VAAccountID, "1000000000")
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
