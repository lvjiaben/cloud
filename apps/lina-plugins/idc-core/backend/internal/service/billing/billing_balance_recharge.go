package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type balanceRechargeContextRow struct {
	ContextNo           string     `orm:"context_no"`
	PaymentAttemptNo    string     `orm:"payment_attempt_no"`
	CustomerNo          string     `orm:"customer_no"`
	PrincipalRefDigest  string     `orm:"principal_ref_digest"`
	CurrencyCode        string     `orm:"currency_code"`
	AmountMinor         int64      `orm:"amount_minor"`
	StatusCode          string     `orm:"status_code"`
	ContentDigest       string     `orm:"content_digest"`
	LedgerTransactionNo string     `orm:"ledger_transaction_no"`
	ExpiresAt           time.Time  `orm:"expires_at"`
	ConfirmedAt         *time.Time `orm:"confirmed_at"`
}

func (s *channelPaymentOwnerService) PrepareBalanceRecharge(ctx context.Context, input billingcap.PrepareBalanceRecharge) (*billingcap.PreparedBalanceRecharge, error) {
	if err := billingcap.ValidatePrepareBalanceRecharge(input); err != nil {
		return nil, err
	}
	if s.principal == nil {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive || audience.UserID <= 0 || audience.CustomerNo == "" {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	if err = s.ledger.requireActiveCustomer(ctx, audience.CustomerNo); err != nil {
		return nil, err
	}
	contextNo := balanceRechargeContextNo(input.PaymentAttemptNo)
	principalDigest := billingcap.PrincipalReferenceDigest(audience.UserID)
	digest, err := contentDigest(struct {
		AttemptNo, ContextNo, CustomerNo, PrincipalDigest, Currency string
		AmountMinor                                                 int64
	}{input.PaymentAttemptNo, contextNo, audience.CustomerNo, principalDigest, input.CurrencyCode, input.AmountMinor})
	if err != nil {
		return nil, err
	}
	var result *billingcap.PreparedBalanceRecharge
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, loadErr := databaseTime(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		expiresAt := now.Add(30 * time.Minute).UTC()
		if _, loadErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_balance_recharge_context(tenant_id,context_no,payment_attempt_no,customer_no,principal_ref_digest,currency_code,amount_minor,status_code,content_digest,expires_at) VALUES(0,?,?,?,?,?,?,'prepared',?,?) ON CONFLICT(payment_attempt_no) DO NOTHING`, contextNo, input.PaymentAttemptNo, audience.CustomerNo, principalDigest, input.CurrencyCode, input.AmountMinor, digest, expiresAt); loadErr != nil {
			return loadErr
		}
		var row *balanceRechargeContextRow
		if loadErr = s.database.GetScan(ctx, &row, `SELECT context_no,payment_attempt_no,customer_no,principal_ref_digest,currency_code,amount_minor,status_code,content_digest,ledger_transaction_no,expires_at,confirmed_at FROM plugin_idc_core_balance_recharge_context WHERE tenant_id=0 AND payment_attempt_no=? FOR UPDATE`, input.PaymentAttemptNo); loadErr != nil {
			return loadErr
		}
		if row == nil || row.ContextNo != contextNo || row.CustomerNo != audience.CustomerNo || row.PrincipalRefDigest != principalDigest || row.CurrencyCode != input.CurrencyCode || row.AmountMinor != input.AmountMinor || row.ContentDigest != digest || row.StatusCode != "prepared" || !row.ExpiresAt.After(now) {
			return bizerr.NewCode(CodeConflict)
		}
		result = &billingcap.PreparedBalanceRecharge{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: row.ContextNo, CustomerRefDigest: row.PrincipalRefDigest, CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor, ExpiresAt: row.ExpiresAt.UTC()}
		return billingcap.ValidatePreparedBalanceRecharge(*result)
	})
	return result, normalizeInvoiceError(err)
}

func (s *channelPaymentOwnerService) ConfirmBalanceRecharge(ctx context.Context, input billingcap.ConfirmedBalanceRecharge) (*billingcap.BalanceRechargeResult, error) {
	if err := billingcap.ValidateConfirmedBalanceRecharge(input); err != nil {
		return nil, err
	}
	var result *billingcap.BalanceRechargeResult
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row *balanceRechargeContextRow
		if loadErr := s.database.GetScan(ctx, &row, `SELECT context_no,payment_attempt_no,customer_no,principal_ref_digest,currency_code,amount_minor,status_code,content_digest,ledger_transaction_no,expires_at,confirmed_at FROM plugin_idc_core_balance_recharge_context WHERE tenant_id=0 AND context_no=? FOR UPDATE`, input.RechargeContextNo); loadErr != nil {
			return loadErr
		}
		if row == nil || row.PaymentAttemptNo != input.PaymentAttemptNo || row.CurrencyCode != input.CurrencyCode || row.AmountMinor != input.AmountMinor || (row.StatusCode != "prepared" && row.StatusCode != "confirmed") {
			return bizerr.NewCode(CodeConflict)
		}
		posting, loadErr := s.ledger.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor, SourceReference: input.PaymentAttemptNo, SourceSnapshotDigest: input.EvidenceDigest})
		if loadErr != nil {
			return loadErr
		}
		if row.StatusCode == "prepared" {
			updated, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_balance_recharge_context SET status_code='confirmed',ledger_transaction_no=?,confirmed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND context_no=? AND status_code='prepared'`, posting.TransactionNo, row.ContextNo)
			if updateErr != nil {
				return updateErr
			}
			affected, _ := updated.RowsAffected()
			if affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		} else if row.LedgerTransactionNo != posting.TransactionNo {
			return bizerr.NewCode(CodeConflict)
		}
		result = &billingcap.BalanceRechargeResult{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: row.ContextNo, CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor, TransactionNo: posting.TransactionNo, BalanceMinor: posting.BalanceMinor, AvailableMinor: posting.AvailableBalanceMinor, Reused: row.StatusCode == "confirmed" || posting.Reused}
		return billingcap.ValidateBalanceRechargeResult(*result)
	})
	if err != nil {
		g.Log().Warningf(ctx, "balance recharge confirmation failed: attempt=%s context=%s error=%v", input.PaymentAttemptNo, input.RechargeContextNo, err)
	}
	return result, normalizeInvoiceError(err)
}

func balanceRechargeContextNo(attemptNo string) string {
	digest := sha256.Sum256([]byte("balance-recharge-context\x00" + attemptNo))
	return "BRC_" + hex.EncodeToString(digest[:16])
}
