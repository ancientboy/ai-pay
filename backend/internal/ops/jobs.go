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
