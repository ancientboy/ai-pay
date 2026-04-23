package ops

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-pay-backend/internal/storage"
)

type Jobs struct {
	store             *storage.Store
	httpClient        *http.Client
	webhookSigningKey string
}

type ReconcileResult struct {
	CheckedAccounts int
	Mismatched      int
}

func NewJobs(store *storage.Store) *Jobs {
	return &Jobs{
		store: store,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (j *Jobs) SetWebhookSigningSecret(secret string) {
	j.webhookSigningKey = strings.TrimSpace(secret)
}

type WebhookDeliveryResult struct {
	Checked int
	Sent    int
	Retried int
	Dead    int
	Skipped int
}

func (j *Jobs) RunStatusCompensation(ctx context.Context) (int64, error) {
	rows, err := j.store.DB.QueryContext(ctx, `
SELECT order_id, hold_id
FROM pay_order
WHERE status = 'SETTLING'
  AND created_at < UTC_TIMESTAMP() - INTERVAL 5 MINUTE`)
	if err != nil {
		return 0, fmt.Errorf("status compensation query failed: %w", err)
	}
	defer rows.Close()
	affected := int64(0)
	for rows.Next() {
		var orderID string
		var holdID sql.NullString
		if err := rows.Scan(&orderID, &holdID); err != nil {
			return affected, fmt.Errorf("status compensation scan failed: %w", err)
		}
		tx, err := j.store.DB.BeginTx(ctx, nil)
		if err != nil {
			return affected, fmt.Errorf("status compensation begin tx failed: %w", err)
		}
		if holdID.Valid && holdID.String != "" {
			var amount float64
			var vaID string
			err = tx.QueryRowContext(ctx, `
SELECT amount, va_account_id
FROM account_hold
WHERE hold_id = ? AND status = 'FROZEN'
FOR UPDATE`, holdID.String).Scan(&amount, &vaID)
			if err != nil && err != sql.ErrNoRows {
				tx.Rollback()
				return affected, fmt.Errorf("status compensation query hold failed: %w", err)
			}
			if err == nil {
				if _, err := tx.ExecContext(ctx, `
UPDATE asset_va_account
SET frozen_balance = frozen_balance - ?, balance = balance + ?
WHERE va_account_id = ? AND frozen_balance >= ?`, amount, amount, vaID, amount); err != nil {
					tx.Rollback()
					return affected, fmt.Errorf("status compensation release amount failed: %w", err)
				}
				if _, err := tx.ExecContext(ctx, `
UPDATE account_hold
SET status = 'RELEASED', updated_at = UTC_TIMESTAMP()
WHERE hold_id = ?`, holdID.String); err != nil {
					tx.Rollback()
					return affected, fmt.Errorf("status compensation update hold failed: %w", err)
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE pay_order
SET status = 'FAILED'
WHERE order_id = ?`, orderID); err != nil {
			tx.Rollback()
			return affected, fmt.Errorf("status compensation update order failed: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return affected, fmt.Errorf("status compensation commit failed: %w", err)
		}
		affected++
	}
	if affected > 0 {
		log.Printf("[ALERT] compensation changed %d stale settling orders to FAILED", affected)
	}
	return affected, nil
}

func (j *Jobs) RunDailyReconcile(ctx context.Context, day time.Time) (ReconcileResult, error) {
	rows, err := j.store.DB.QueryContext(ctx, `SELECT va_account_id, balance, frozen_balance FROM asset_va_account`)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("query accounts failed: %w", err)
	}
	defer rows.Close()

	result := ReconcileResult{}
	for rows.Next() {
		var vaID string
		var balance float64
		var frozenBalance float64
		if err := rows.Scan(&vaID, &balance, &frozenBalance); err != nil {
			return ReconcileResult{}, err
		}
		result.CheckedAccounts++

		var recharge float64
		if err := j.store.DB.QueryRowContext(ctx, `
SELECT COALESCE(SUM(amount),0) FROM fund_recharge_order
WHERE va_account_id = ? AND status = 'SETTLED'`, vaID).Scan(&recharge); err != nil {
			return ReconcileResult{}, err
		}

		var debit sql.NullFloat64
		if err := j.store.DB.QueryRowContext(ctx, `
SELECT SUM(p.amount) FROM pay_order p
JOIN asset_va_account a ON a.agent_did = p.agent_did
WHERE a.va_account_id = ? AND p.status = 'SETTLED'`, vaID).Scan(&debit); err != nil {
			return ReconcileResult{}, err
		}
		paid := 0.0
		if debit.Valid {
			paid = debit.Float64
		}
		expected := recharge - paid
		if !closeEnough(expected, balance+frozenBalance) {
			result.Mismatched++
			log.Printf("[ALERT] reconcile mismatch va=%s expected=%.8f actual_total=%.8f day=%s", vaID, expected, balance+frozenBalance, day.UTC().Format("2006-01-02"))
		}
	}
	return result, nil
}

func closeEnough(a, b float64) bool {
	const eps = 0.0000001
	if a > b {
		return a-b < eps
	}
	return b-a < eps
}

func (j *Jobs) RunWebhookDelivery(ctx context.Context, now time.Time, limit int) (WebhookDeliveryResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := j.store.DB.QueryContext(ctx, `
SELECT id, webhook_id, url, event, payload, attempts, max_attempts
FROM webhook_delivery_task
WHERE status IN ('PENDING', 'RETRYING')
  AND next_retry_at <= ?
ORDER BY created_at ASC
LIMIT ?`, now.UTC(), limit)
	if err != nil {
		return WebhookDeliveryResult{}, fmt.Errorf("query webhook tasks failed: %w", err)
	}
	defer rows.Close()
	result := WebhookDeliveryResult{}
	for rows.Next() {
		result.Checked++
		var id int64
		var webhookID, url, event string
		var payloadRaw string
		var attempts, maxAttempts int
		if err := rows.Scan(&id, &webhookID, &url, &event, &payloadRaw, &attempts, &maxAttempts); err != nil {
			return result, fmt.Errorf("scan webhook task failed: %w", err)
		}
		if strings.TrimSpace(url) == "" {
			result.Skipped++
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(payloadRaw))
		if err != nil {
			if err := j.markWebhookTaskFailure(ctx, id, attempts, maxAttempts, "build request failed", now); err != nil {
				return result, err
			}
			result.Retried++
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Event", event)
		req.Header.Set("X-Webhook-Delivery-Id", fmt.Sprintf("%d", id))
		ts := now.UTC().Format(time.RFC3339)
		nonce := fmt.Sprintf("wn_%d", time.Now().UnixNano())
		attempt := attempts + 1
		req.Header.Set("X-Webhook-Timestamp", ts)
		req.Header.Set("X-Webhook-Nonce", nonce)
		req.Header.Set("X-Webhook-Attempt", fmt.Sprintf("%d", attempt))
		if j.webhookSigningKey != "" {
			sign := buildWebhookSignatureV1(j.webhookSigningKey, id, event, ts, nonce, attempt, payloadRaw)
			req.Header.Set("X-Webhook-Signature-Version", "v1")
			req.Header.Set("X-Webhook-Signature", sign)
		}
		resp, err := j.httpClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if _, err := j.store.DB.ExecContext(ctx, `
UPDATE webhook_delivery_task
SET status = 'SENT', attempts = attempts + 1, last_error = NULL, updated_at = UTC_TIMESTAMP()
WHERE id = ?`, id); err != nil {
				return result, fmt.Errorf("mark webhook sent failed: %w", err)
			}
			result.Sent++
			continue
		}
		lastErr := "deliver failed"
		if err != nil {
			lastErr = err.Error()
		} else {
			lastErr = fmt.Sprintf("status %d", resp.StatusCode)
		}
		if err := j.markWebhookTaskFailure(ctx, id, attempts, maxAttempts, lastErr, now); err != nil {
			return result, err
		}
		if attempts+1 >= maxAttempts {
			result.Dead++
		} else {
			result.Retried++
		}
	}
	return result, nil
}

func (j *Jobs) markWebhookTaskFailure(ctx context.Context, id int64, attempts, maxAttempts int, lastErr string, now time.Time) error {
	nextAttempt := attempts + 1
	if nextAttempt >= maxAttempts {
		if _, err := j.store.DB.ExecContext(ctx, `
UPDATE webhook_delivery_task
SET status = 'DEAD', attempts = ?, last_error = ?, updated_at = UTC_TIMESTAMP()
WHERE id = ?`, nextAttempt, truncateText(lastErr, 255), id); err != nil {
			return fmt.Errorf("mark webhook dead failed: %w", err)
		}
		return nil
	}
	backoffSeconds := 1 << minInt(nextAttempt, 6)
	nextRetry := now.UTC().Add(time.Duration(backoffSeconds) * time.Second)
	if _, err := j.store.DB.ExecContext(ctx, `
UPDATE webhook_delivery_task
SET status = 'RETRYING', attempts = ?, next_retry_at = ?, last_error = ?, updated_at = UTC_TIMESTAMP()
WHERE id = ?`, nextAttempt, nextRetry, truncateText(lastErr, 255), id); err != nil {
		return fmt.Errorf("mark webhook retry failed: %w", err)
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

func buildWebhookSignatureV1(secret string, deliveryID int64, event, ts, nonce string, attempt int, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	payload := fmt.Sprintf("%d|%s|%s|%s|%d|%s",
		deliveryID,
		strings.TrimSpace(event),
		strings.TrimSpace(ts),
		strings.TrimSpace(nonce),
		attempt,
		body,
	)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
