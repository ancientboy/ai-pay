package ops

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ai-pay-backend/internal/service"
	"ai-pay-backend/internal/storage"
)

func newStoreForOpsIT(t *testing.T) *storage.Store {
	t.Helper()
	dsn := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		t.Skip("skip ops integration test: MYSQL_DSN or REDIS_ADDR is empty")
	}
	store, err := storage.New(context.Background(), storage.Config{
		MySQLDSN:      dsn,
		RedisAddr:     redisAddr,
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
	})
	if err != nil {
		t.Skipf("skip ops integration test: init store failed: %v", err)
	}
	return store
}

func TestRunDailyReconcileIntegration(t *testing.T) {
	store := newStoreForOpsIT(t)
	defer store.Close()

	svc := service.NewPersistent(store)
	did := fmt.Sprintf("did:gusd:agent:it_reconcile_%d", time.Now().UnixNano())
	_ = svc.RegisterAgent(did)
	acc := svc.CreateAccount(did)
	if err := svc.Recharge(acc.VAAccountID, "GUSD", "5", fmt.Sprintf("rch-it-reconcile-%d", time.Now().UnixNano())); err != nil {
		t.Fatalf("recharge failed: %v", err)
	}

	jobs := NewJobs(store)
	result, err := jobs.RunDailyReconcile(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("run reconcile failed: %v", err)
	}
	if result.CheckedAccounts < 1 {
		t.Fatalf("expected checked accounts >=1 got %d", result.CheckedAccounts)
	}
}

func ensureWebhookDeliveryTaskTable(t *testing.T, store *storage.Store) {
	t.Helper()
	_, err := store.DB.Exec(`
CREATE TABLE IF NOT EXISTS webhook_delivery_task (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  webhook_id VARCHAR(64) NOT NULL,
  url VARCHAR(512) NOT NULL,
  event VARCHAR(128) NOT NULL,
  dedupe_key VARCHAR(128) NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5,
  next_retry_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_error VARCHAR(255) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_webhook_delivery_dedupe (webhook_id, event, dedupe_key),
  INDEX idx_webhook_delivery_status_retry (status, next_retry_at)
)`)
	if err != nil {
		t.Fatalf("ensure webhook_delivery_task failed: %v", err)
	}
}

func TestRunWebhookDeliveryIntegration(t *testing.T) {
	store := newStoreForOpsIT(t)
	defer store.Close()
	ensureWebhookDeliveryTaskTable(t, store)

	var gotEvent string
	var gotDeliveryID string
	var gotTimestamp string
	var gotNonce string
	var gotAttempt string
	var gotVersion string
	var gotSignature string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-Webhook-Event")
		gotDeliveryID = r.Header.Get("X-Webhook-Delivery-Id")
		gotTimestamp = r.Header.Get("X-Webhook-Timestamp")
		gotNonce = r.Header.Get("X-Webhook-Nonce")
		gotAttempt = r.Header.Get("X-Webhook-Attempt")
		gotVersion = r.Header.Get("X-Webhook-Signature-Version")
		gotSignature = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	webhookID := fmt.Sprintf("wh_it_%d", time.Now().UnixNano())
	dedupe := fmt.Sprintf("tx:it_%d", time.Now().UnixNano())
	if _, err := store.DB.Exec(`
INSERT INTO webhook_delivery_task (webhook_id, url, event, dedupe_key, payload, status, attempts, max_attempts, next_retry_at)
VALUES (?, ?, 'payment.settled', ?, JSON_OBJECT('transactionId', ?), 'PENDING', 0, 3, UTC_TIMESTAMP())`,
		webhookID, srv.URL, dedupe, dedupe); err != nil {
		t.Fatalf("seed webhook task failed: %v", err)
	}

	jobs := NewJobs(store)
	jobs.SetWebhookSigningSecret("it_webhook_sign")
	result, err := jobs.RunWebhookDelivery(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("run webhook delivery failed: %v", err)
	}
	if result.Sent != 1 {
		t.Fatalf("expected sent=1 got %+v", result)
	}
	if gotEvent != "payment.settled" {
		t.Fatalf("expected X-Webhook-Event header")
	}
	if gotDeliveryID == "" || gotTimestamp == "" || gotNonce == "" || gotAttempt != "1" || gotVersion != "v1" || gotSignature == "" {
		t.Fatalf("missing webhook signature headers")
	}

	var status string
	var attempts int
	if err := store.DB.QueryRow(`SELECT status, attempts FROM webhook_delivery_task WHERE webhook_id = ?`, webhookID).Scan(&status, &attempts); err != nil {
		t.Fatalf("query webhook task failed: %v", err)
	}
	if status != "SENT" || attempts != 1 {
		t.Fatalf("unexpected delivery row status=%s attempts=%d", status, attempts)
	}
}

func TestRunWebhookDeliveryMovesToDeadLetter(t *testing.T) {
	store := newStoreForOpsIT(t)
	defer store.Close()
	ensureWebhookDeliveryTaskTable(t, store)

	webhookID := fmt.Sprintf("wh_dead_%d", time.Now().UnixNano())
	dedupe := fmt.Sprintf("tx:dead_%d", time.Now().UnixNano())
	if _, err := store.DB.Exec(`
INSERT INTO webhook_delivery_task (webhook_id, url, event, dedupe_key, payload, status, attempts, max_attempts, next_retry_at)
VALUES (?, ?, 'payment.refunded', ?, JSON_OBJECT('transactionId', ?), 'PENDING', 0, 1, UTC_TIMESTAMP())`,
		webhookID, "http://127.0.0.1:1/unreachable", dedupe, dedupe); err != nil {
		t.Fatalf("seed dead-letter task failed: %v", err)
	}

	jobs := NewJobs(store)
	result, err := jobs.RunWebhookDelivery(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("run webhook delivery failed: %v", err)
	}
	if result.Dead != 1 {
		t.Fatalf("expected dead=1 got %+v", result)
	}

	var status string
	var attempts int
	var lastErr string
	if err := store.DB.QueryRow(`SELECT status, attempts, COALESCE(last_error, '') FROM webhook_delivery_task WHERE webhook_id = ?`, webhookID).
		Scan(&status, &attempts, &lastErr); err != nil {
		t.Fatalf("query dead-letter task failed: %v", err)
	}
	if status != "DEAD" || attempts != 1 || lastErr == "" {
		t.Fatalf("unexpected dead-letter row status=%s attempts=%d lastErr=%q", status, attempts, lastErr)
	}
}
