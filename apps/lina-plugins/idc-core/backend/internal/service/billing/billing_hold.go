package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type holdRow struct {
	HoldNo              string    `orm:"hold_no"`
	CustomerNo          string    `orm:"customer_no"`
	AccountNo           string    `orm:"account_no"`
	CurrencyCode        string    `orm:"currency_code"`
	SourceType          string    `orm:"source_type_code"`
	SourceReference     string    `orm:"source_reference"`
	AmountMinor         int64     `orm:"amount_minor"`
	ConsumedAmountMinor int64     `orm:"consumed_amount_minor"`
	ReleasedAmountMinor int64     `orm:"released_amount_minor"`
	StatusCode          string    `orm:"status_code"`
	ContentDigest       string    `orm:"content_digest"`
	ExpiresAt           time.Time `orm:"expires_at"`
}

type holdSemantic struct {
	CustomerNo           string `json:"customer_no"`
	AccountNo            string `json:"account_no"`
	CurrencyCode         string `json:"currency_code"`
	SourceType           string `json:"source_type"`
	SourceReference      string `json:"source_reference"`
	SourceSnapshotDigest string `json:"source_snapshot_digest"`
	AmountMinor          int64  `json:"amount_minor"`
	ExpiresAt            string `json:"expires_at"`
}

func (s *serviceImpl) CreateHold(ctx context.Context, input HoldCreateInput) (*Hold, error) {
	normalized, err := normalizeHoldCreate(input)
	if err != nil {
		return nil, err
	}
	var result *Hold
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, "hold."+normalized.SourceType, normalized.SourceReference); lockErr != nil {
			return lockErr
		}
		if customerErr := s.requireActiveCustomer(ctx, normalized.CustomerNo); customerErr != nil {
			return customerErr
		}
		balanceSpec := customerBalanceSpec(normalized.CustomerNo)
		accounts, accountErr := s.ensureAndLockAccounts(ctx, normalized.CurrencyCode, true, balanceSpec)
		if accountErr != nil {
			return accountErr
		}
		accountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference,
			balanceSpec.AccountType, normalized.CurrencyCode,
		)
		if accounts[accountNo] == nil {
			return bizerr.NewCode(CodeIntegrity)
		}
		digest, digestErr := contentDigest(holdSemantic{
			CustomerNo: normalized.CustomerNo, AccountNo: accountNo,
			CurrencyCode: normalized.CurrencyCode, SourceType: normalized.SourceType,
			SourceReference:      normalized.SourceReference,
			SourceSnapshotDigest: normalized.SourceSnapshotDigest,
			AmountMinor:          normalized.AmountMinor,
			ExpiresAt:            normalized.ExpiresAt.Format(time.RFC3339Nano),
		})
		if digestErr != nil {
			return digestErr
		}
		existingRows := make([]holdRow, 0, 2)
		if scanErr := s.database.GetScan(ctx, &existingRows, `
SELECT hold_no, customer_no, account_no, currency_code, source_type_code,
       source_reference, amount_minor, consumed_amount_minor, released_amount_minor,
       status_code, content_digest, expires_at
FROM plugin_idc_core_billing_fund_hold
WHERE tenant_id = 0 AND source_type_code = ? AND source_reference = ?
ORDER BY id DESC LIMIT 2 FOR UPDATE`, normalized.SourceType,
			normalized.SourceReference); scanErr != nil {
			return gerror.Wrap(scanErr, "load billing fund hold by source")
		}
		if len(existingRows) > 1 {
			return bizerr.NewCode(CodeIntegrity)
		}
		if len(existingRows) == 1 {
			existing := existingRows[0]
			if existing.ContentDigest != digest || existing.CustomerNo != normalized.CustomerNo ||
				existing.CurrencyCode != normalized.CurrencyCode || existing.AmountMinor != normalized.AmountMinor ||
				!existing.ExpiresAt.UTC().Equal(normalized.ExpiresAt) {
				return bizerr.NewCode(CodeConflict)
			}
			result = projectHold(&existing, true)
			return nil
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if !normalized.ExpiresAt.After(now) {
			return bizerr.NewCode(CodeInvalid)
		}
		balance, held, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, accountNo)
		if balanceErr != nil {
			return balanceErr
		}
		if balance-held < normalized.AmountMinor {
			return bizerr.NewCode(CodeInsufficientBalance)
		}
		holdNo, numberErr := newBillingNumber(s.random, "bhd_")
		if numberErr != nil {
			return numberErr
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_fund_hold (
    tenant_id, hold_no, customer_no, account_no, account_type_code, currency_code,
    source_type_code, source_reference, amount_minor, content_digest, expires_at
) VALUES (0, ?, ?, ?, 'customer_balance', ?, ?, ?, ?, ?, ?)`,
			holdNo, normalized.CustomerNo, accountNo, normalized.CurrencyCode,
			normalized.SourceType, normalized.SourceReference, normalized.AmountMinor,
			digest, normalized.ExpiresAt); insertErr != nil {
			return gerror.Wrap(insertErr, "insert billing fund hold")
		}
		result = &Hold{
			HoldNo: holdNo, CustomerNo: normalized.CustomerNo,
			CurrencyCode: normalized.CurrencyCode, AmountMinor: normalized.AmountMinor,
			RemainingAmountMinor: normalized.AmountMinor, Status: "active",
			ExpiresAt: normalized.ExpiresAt,
		}
		return nil
	})
	return result, err
}

func (s *serviceImpl) ConsumeHold(ctx context.Context, input HoldConsumeInput) (*Posting, error) {
	normalized, err := normalizeHoldConsume(input)
	if err != nil {
		return nil, err
	}
	var result *Posting
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, sourceHoldConsumption, normalized.SourceReference); lockErr != nil {
			return lockErr
		}
		existing, existingErr := loadTransactionBySource(
			ctx, s.database, sourceHoldConsumption, normalized.SourceReference,
		)
		if existingErr != nil {
			return existingErr
		}
		preloaded, loadErr := loadHold(ctx, s.database, normalized.HoldNo, false)
		if loadErr != nil {
			return loadErr
		}
		if preloaded == nil {
			return bizerr.NewCode(CodeNotFound)
		}
		if existing == nil {
			if customerErr := s.requireActiveCustomer(ctx, preloaded.CustomerNo); customerErr != nil {
				return customerErr
			}
		}
		merchantNo, merchantErr := s.loadMerchantNo(ctx, existing == nil)
		if merchantErr != nil {
			return merchantErr
		}
		balanceSpec := customerBalanceSpec(preloaded.CustomerNo)
		revenueSpec := merchantRevenueSpec(merchantNo)
		accounts, accountErr := s.ensureAndLockAccounts(
			ctx, preloaded.CurrencyCode, existing == nil, balanceSpec, revenueSpec,
		)
		if accountErr != nil {
			return accountErr
		}
		hold, holdErr := loadHold(ctx, s.database, normalized.HoldNo, true)
		if holdErr != nil {
			return holdErr
		}
		if hold == nil || hold.CustomerNo != preloaded.CustomerNo ||
			hold.AccountNo != preloaded.AccountNo || hold.CurrencyCode != preloaded.CurrencyCode {
			return bizerr.NewCode(CodeIntegrity)
		}
		revenueAccountNo := deterministicAccountNo(
			revenueSpec.OwnerType, revenueSpec.OwnerReference,
			revenueSpec.AccountType, hold.CurrencyCode,
		)
		entries := []entrySemantic{
			{AccountNo: hold.AccountNo, Direction: "debit", Role: "balance",
				AmountMinor: normalized.AmountMinor, InvoiceNo: normalized.InvoiceNo, Reason: "balance_hold_consumption"},
			{AccountNo: revenueAccountNo, Direction: "credit", Role: "revenue",
				AmountMinor: normalized.AmountMinor, InvoiceNo: normalized.InvoiceNo, Reason: "balance_hold_consumption"},
		}
		if existing == nil {
			now, nowErr := databaseTime(ctx, s.database)
			if nowErr != nil {
				return nowErr
			}
			remaining := hold.AmountMinor - hold.ConsumedAmountMinor - hold.ReleasedAmountMinor
			if (hold.StatusCode != "active" && hold.StatusCode != "partially_consumed") ||
				!hold.ExpiresAt.After(now) || remaining < normalized.AmountMinor {
				return bizerr.NewCode(CodeConflict)
			}
		}
		transaction, _, postErr := s.postLocked(
			ctx, "balance_payment", hold.CurrencyCode,
			sourceHoldConsumption, normalized.SourceReference,
			normalized.SourceSnapshotDigest, "", entries, accounts,
		)
		if postErr != nil {
			return postErr
		}
		if existing == nil {
			newConsumed := hold.ConsumedAmountMinor + normalized.AmountMinor
			status := "partially_consumed"
			if newConsumed == hold.AmountMinor {
				status = "consumed"
			}
			if _, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_billing_fund_hold
SET consumed_amount_minor = ?, status_code = ?,
    consumed_at = CASE WHEN ? = 'consumed' THEN CURRENT_TIMESTAMP ELSE NULL END,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND hold_no = ? AND version > 0`,
				newConsumed, status, status, hold.HoldNo); updateErr != nil {
				return gerror.Wrap(updateErr, "consume billing fund hold")
			}
		}
		balance, heldAmount, balanceErr := loadCurrentBalanceAndHeld(ctx, s.database, hold.AccountNo)
		if balanceErr != nil {
			return balanceErr
		}
		result = &Posting{
			TransactionNo: transaction.TransactionNo, TransactionType: "balance_payment",
			CurrencyCode: hold.CurrencyCode, AmountMinor: normalized.AmountMinor,
			CustomerNo: hold.CustomerNo, BalanceMinor: balance,
			HeldBalanceMinor: heldAmount, AvailableBalanceMinor: balance - heldAmount,
			Reused: existing != nil,
		}
		return nil
	})
	return result, err
}

func (s *serviceImpl) ReleaseHold(ctx context.Context, holdNo string) (*Hold, error) {
	return s.closeHold(ctx, holdNo, false)
}

func (s *serviceImpl) ExpireHold(ctx context.Context, holdNo string) (*Hold, error) {
	return s.closeHold(ctx, holdNo, true)
}

func (s *serviceImpl) closeHold(ctx context.Context, holdNo string, expiryOnly bool) (*Hold, error) {
	holdNo = strings.TrimSpace(holdNo)
	if !validBusinessReference(holdNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *Hold
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		preloaded, loadErr := loadHold(ctx, s.database, holdNo, false)
		if loadErr != nil {
			return loadErr
		}
		if preloaded == nil {
			return bizerr.NewCode(CodeNotFound)
		}
		if _, accountErr := s.lockExistingAccounts(ctx, []string{preloaded.AccountNo}, false); accountErr != nil {
			return accountErr
		}
		hold, holdErr := loadHold(ctx, s.database, holdNo, true)
		if holdErr != nil {
			return holdErr
		}
		if hold == nil {
			return bizerr.NewCode(CodeIntegrity)
		}
		if hold.StatusCode == "released" || hold.StatusCode == "expired" || hold.StatusCode == "consumed" {
			if expiryOnly && hold.StatusCode != "expired" {
				return bizerr.NewCode(CodeConflict)
			}
			result = projectHold(hold, true)
			return nil
		}
		now, nowErr := databaseTime(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if expiryOnly && hold.ExpiresAt.After(now) {
			return bizerr.NewCode(CodeConflict)
		}
		remaining := hold.AmountMinor - hold.ConsumedAmountMinor - hold.ReleasedAmountMinor
		if remaining <= 0 {
			return bizerr.NewCode(CodeIntegrity)
		}
		status := "released"
		if expiryOnly || !hold.ExpiresAt.After(now) {
			status = "expired"
		}
		if _, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_billing_fund_hold
SET released_amount_minor = released_amount_minor + ?, status_code = ?,
    released_at = CURRENT_TIMESTAMP, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND hold_no = ?`, remaining, status, hold.HoldNo); updateErr != nil {
			return gerror.Wrap(updateErr, "close billing fund hold")
		}
		hold.ReleasedAmountMinor += remaining
		hold.StatusCode = status
		result = projectHold(hold, false)
		return nil
	})
	return result, err
}

func loadHold(ctx context.Context, database gdb.DB, holdNo string, lock bool) (*holdRow, error) {
	query := `
SELECT hold_no, customer_no, account_no, currency_code, source_type_code,
       source_reference, amount_minor, consumed_amount_minor, released_amount_minor,
       status_code, content_digest, expires_at
FROM plugin_idc_core_billing_fund_hold
WHERE tenant_id = 0 AND hold_no = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var row *holdRow
	if err := database.GetScan(ctx, &row, query, holdNo); err != nil {
		return nil, gerror.Wrap(err, "load billing fund hold")
	}
	return row, nil
}

func projectHold(row *holdRow, reused bool) *Hold {
	return &Hold{
		HoldNo: row.HoldNo, CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode,
		AmountMinor: row.AmountMinor, ConsumedAmountMinor: row.ConsumedAmountMinor,
		ReleasedAmountMinor:  row.ReleasedAmountMinor,
		RemainingAmountMinor: row.AmountMinor - row.ConsumedAmountMinor - row.ReleasedAmountMinor,
		Status:               row.StatusCode, ExpiresAt: row.ExpiresAt.UTC(), Reused: reused,
	}
}

func databaseTime(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read billing database time")
	}
	return value.Time().UTC(), nil
}
