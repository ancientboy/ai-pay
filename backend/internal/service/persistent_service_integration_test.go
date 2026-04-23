package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"ai-pay-backend/internal/storage"
)

func newPersistentForIT(t *testing.T) (*PersistentService, *storage.Store) {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		t.Skip("skip persistent integration test: MYSQL_DSN or REDIS_ADDR is empty")
	}
	store, err := storage.New(context.Background(), storage.Config{
		MySQLDSN:      dsn,
		RedisAddr:     redisAddr,
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
	})
	if err != nil {
		t.Skipf("skip persistent integration test: init store failed: %v", err)
	}
	return NewPersistent(store), store
}

func TestPersistentRechargeIdempotentByVACardNo(t *testing.T) {
	svc, store := newPersistentForIT(t)
	defer store.Close()

	did := fmt.Sprintf("did:gusd:agent:it_persist_%d", time.Now().UnixNano())
	_ = svc.RegisterAgent(did)
	acc := svc.CreateAccount(did)
	if acc.VACardNo == "" {
		t.Fatalf("expected va card no")
	}

	idem := fmt.Sprintf("rch-it-idem-%d", time.Now().UnixNano())
	if err := svc.Recharge(acc.VACardNo, "10", idem); err != nil {
		t.Fatalf("first recharge failed: %v", err)
	}
	if err := svc.Recharge(acc.VACardNo, "10", idem); err != nil {
		t.Fatalf("second recharge with same idem should be idempotent: %v", err)
	}
	balance, err := svc.BalanceByVA(acc.VAAccountID)
	if err != nil {
		t.Fatalf("query balance failed: %v", err)
	}
	if balance != 10 {
		t.Fatalf("expected idempotent balance=10 got %v", balance)
	}
}

func TestPersistentPayAndQueryStatus(t *testing.T) {
	svc, store := newPersistentForIT(t)
	defer store.Close()

	did := fmt.Sprintf("did:gusd:agent:it_pay_%d", time.Now().UnixNano())
	_ = svc.RegisterAgent(did)
	acc := svc.CreateAccount(did)
	if err := svc.Recharge(acc.VAAccountID, "30", fmt.Sprintf("rch-it-pay-%d", time.Now().UnixNano())); err != nil {
		t.Fatalf("recharge failed: %v", err)
	}
	if err := svc.SetAuthorizeRule(did, "10", "50", []string{"m1"}); err != nil {
		t.Fatalf("set authorize failed: %v", err)
	}

	resp, apiErr := svc.Pay(PayRequest{
		PayerDID:       did,
		MerchantID:     "m1",
		Amount:         "3",
		IdempotencyKey: fmt.Sprintf("idem-it-pay-%d", time.Now().UnixNano()),
		Signature:      "sig",
	})
	if apiErr != nil {
		t.Fatalf("pay failed: %v", apiErr)
	}
	tx, err := svc.QueryStatus(resp.TransactionID)
	if err != nil {
		t.Fatalf("query status failed: %v", err)
	}
	if tx.Status == "" {
		t.Fatalf("expected non-empty status")
	}
}
