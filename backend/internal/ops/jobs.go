package ops

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"ai-pay-backend/internal/storage"
)

type Jobs struct {
	store *storage.Store
}

type ReconcileResult struct {
	CheckedAccounts int
	Mismatched      int
}

func NewJobs(store *storage.Store) *Jobs {
	return &Jobs{store: store}
}

func (j *Jobs) RunStatusCompensation(ctx context.Context) (int64, error) {
	res, err := j.store.DB.ExecContext(ctx, `
UPDATE pay_order
SET status = 'FAILED'
WHERE status = 'SETTLING'
  AND created_at < UTC_TIMESTAMP() - INTERVAL 5 MINUTE`)
	if err != nil {
		return 0, fmt.Errorf("status compensation update failed: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected > 0 {
		log.Printf("[ALERT] compensation changed %d stale settling orders to FAILED", affected)
	}
	return affected, nil
}

func (j *Jobs) RunDailyReconcile(ctx context.Context, day time.Time) (ReconcileResult, error) {
	rows, err := j.store.DB.QueryContext(ctx, `SELECT va_account_id, balance FROM asset_va_account`)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("query accounts failed: %w", err)
	}
	defer rows.Close()

	result := ReconcileResult{}
	for rows.Next() {
		var vaID string
		var balance float64
		if err := rows.Scan(&vaID, &balance); err != nil {
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
		if !closeEnough(expected, balance) {
			result.Mismatched++
			log.Printf("[ALERT] reconcile mismatch va=%s expected=%.8f actual=%.8f day=%s", vaID, expected, balance, day.UTC().Format("2006-01-02"))
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
