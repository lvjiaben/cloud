// This file records offline commission payments and their ledger movement in
// one transaction. It never calls a payment provider or spending balance owner.
package salesmanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	commissionEntryPayout         = "payout"
	commissionSourceOfflinePayout = "offline_payout"
)

var _ CommissionService = (*serviceImpl)(nil)

func (s *serviceImpl) RecordCommissionPayout(ctx context.Context, input CommissionPayoutInput) (CommissionPayout, error) {
	actor, err := s.authorize(ctx, permissionCommissionPayout, false)
	if err != nil {
		return CommissionPayout{}, err
	}
	input.AccountNo = strings.TrimSpace(input.AccountNo)
	input.EvidenceReference = strings.TrimSpace(input.EvidenceReference)
	amount, err := strconv.ParseInt(input.AmountMinor, 10, 64)
	if err != nil || amount <= 0 || minorString(amount) != input.AmountMinor || input.ExpectedVersion < 1 || input.AccountNo == "" || len(input.AccountNo) > 40 || utf8.RuneCountInString(input.EvidenceReference) < 6 || utf8.RuneCountInString(input.EvidenceReference) > 512 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 || input.PaidAt.IsZero() {
		return CommissionPayout{}, bizerr.NewCode(codeInvalid)
	}
	payload, err := json.Marshal(struct {
		Account, Amount, Evidence string
		PaidAt                    int64
		Version                   int64
	}{input.AccountNo, input.AmountMinor, input.EvidenceReference, input.PaidAt.UnixMilli(), input.ExpectedVersion})
	if err != nil {
		return CommissionPayout{}, managementError(err)
	}
	digest := commissionDigest(string(payload))
	payoutNo := "COP_" + commissionDigest(strconv.Itoa(actor) + "\x00" + input.IdempotencyKey)[:32]
	var result CommissionPayout
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var account *entity.SalesCommissionAccount
		if err := s.database.Model(dao.SalesCommissionAccount.Table()).Ctx(ctx).Where(do.SalesCommissionAccount{TenantId: 0, AccountNo: input.AccountNo}).LockUpdate().Scan(&account); err != nil {
			return err
		}
		if account == nil {
			return bizerr.NewCode(codeConflict)
		}
		var prior *entity.SalesCommissionOfflinePayout
		if err := s.database.Model(dao.SalesCommissionOfflinePayout.Table()).Ctx(ctx).Where(do.SalesCommissionOfflinePayout{TenantId: 0, PayoutNo: payoutNo}).Scan(&prior); err != nil {
			return err
		}
		if prior != nil {
			if prior.RequestDigest != digest {
				return bizerr.NewCode(codeConflict)
			}
			result = payoutProjection(*prior)
			return nil
		}
		recoveries, err := s.commissionRecoveryDue(ctx, []string{account.AccountNo})
		if err != nil {
			return err
		}
		if recoveries[account.AccountNo] > 0 {
			return bizerr.NewCode(codeRecoveryDue)
		}
		var clock struct {
			Now time.Time `orm:"now"`
		}
		if err := s.database.GetScan(ctx, &clock, "SELECT CURRENT_TIMESTAMP AS now"); err != nil {
			return err
		}
		if input.PaidAt.After(clock.Now) || account.Version != input.ExpectedVersion || account.StatusCode != statusActive || account.AvailableAmountMinor < amount {
			return bizerr.NewCode(codeConflict)
		}
		sequence, err := s.database.Model(dao.SalesCommissionAccountEntry.Table()).Ctx(ctx).Where(do.SalesCommissionAccountEntry{TenantId: 0, AccountNo: account.AccountNo}).Max("entry_sequence")
		if err != nil {
			return err
		}
		paidAt := input.PaidAt
		if _, err = s.database.Model(dao.SalesCommissionOfflinePayout.Table()).Ctx(ctx).Data(do.SalesCommissionOfflinePayout{TenantId: 0, PayoutNo: payoutNo, AccountNo: account.AccountNo, CurrencyCode: account.CurrencyCode, AmountMinor: amount, EvidenceReference: input.EvidenceReference, PaidAt: &paidAt, RecordedBy: actor, RequestDigest: digest}).Insert(); err != nil {
			return err
		}
		if err = s.allocateOfflinePayout(ctx, account, payoutNo, amount); err != nil {
			return err
		}
		if _, err = s.database.Model(dao.SalesCommissionAccountEntry.Table()).Ctx(ctx).Data(do.SalesCommissionAccountEntry{TenantId: 0, EntryNo: "CENT_" + commissionDigest(payoutNo)[:32], AccountNo: account.AccountNo, CurrencyCode: account.CurrencyCode, EntrySequence: int64(sequence) + 1, EntryTypeCode: commissionEntryPayout, SourceTypeCode: commissionSourceOfflinePayout, SourceReference: payoutNo, SourceDigest: digest, DeltaCreditedMinor: 0, DeltaPendingMinor: 0, DeltaAvailableMinor: -amount, DeltaFrozenMinor: 0, DeltaPaidMinor: amount, DeltaReversedMinor: 0, AfterCreditedMinor: account.CreditedAmountMinor, AfterPendingMinor: account.PendingAmountMinor, AfterAvailableMinor: account.AvailableAmountMinor - amount, AfterFrozenMinor: account.FrozenAmountMinor, AfterPaidMinor: account.PaidAmountMinor + amount, AfterReversedMinor: account.ReversedAmountMinor}).Insert(); err != nil {
			return err
		}
		result = CommissionPayout{PayoutNo: payoutNo, AccountNo: account.AccountNo, CurrencyCode: account.CurrencyCode, AmountMinor: input.AmountMinor, EvidenceReference: input.EvidenceReference, PaidAt: &paidAt, RecordedBy: actor}
		return s.auditChange(ctx, permissionCommissionPayout, "commission_payout", payoutNo, "", account.Version+1, account, result)
	})
	return result, managementError(err)
}

// allocateOfflinePayout selects a bounded set of matured accruals while the
// account lock serializes payouts and reversals. It writes the allocation in
// one batch; an overlarge payment is rejected without partial ledger changes.
func (s *serviceImpl) allocateOfflinePayout(ctx context.Context, account *entity.SalesCommissionAccount, payoutNo string, amount int64) error {
	var rows []struct {
		CommissionNo string `orm:"commission_no"`
		Available    int64  `orm:"available"`
	}
	if err := s.database.GetScan(ctx, &rows, `
SELECT a.commission_no,
       a.matured_amount_minor - COALESCE(p.amount,0) - COALESCE(l.amount,0) AS available
FROM plugin_idc_core_commission_accrual a
LEFT JOIN (SELECT commission_no,SUM(amount_minor) amount
           FROM plugin_idc_core_sales_commission_offline_payout_item GROUP BY commission_no) p
       ON p.commission_no=a.commission_no
LEFT JOIN (SELECT i.commission_no,SUM(i.amount_minor) amount
           FROM plugin_idc_core_sales_commission_settlement_item i
           JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no
           WHERE s.status_code IN ('pending_review','approved','batched','paid','manual_review')
           GROUP BY i.commission_no) l ON l.commission_no=a.commission_no
WHERE a.tenant_id=0 AND a.beneficiary_type_code=? AND a.beneficiary_reference=?
  AND a.currency_code=? AND a.matured_amount_minor>COALESCE(p.amount,0)+COALESCE(l.amount,0)
ORDER BY a.matures_at,a.id LIMIT 1001`, account.BeneficiaryTypeCode, account.BeneficiaryReference, account.CurrencyCode); err != nil {
		return err
	}
	var allocations []do.SalesCommissionOfflinePayoutItem
	remaining := amount
	for index, row := range rows {
		if remaining == 0 || index == 1000 {
			break
		}
		allocated := min(remaining, row.Available)
		allocations = append(allocations, do.SalesCommissionOfflinePayoutItem{TenantId: 0, PayoutNo: payoutNo, CommissionNo: row.CommissionNo, AmountMinor: allocated})
		remaining -= allocated
	}
	if remaining != 0 {
		return bizerr.NewCode(codeConflict)
	}
	_, err := s.database.Model(dao.SalesCommissionOfflinePayoutItem.Table()).Ctx(ctx).Data(allocations).Insert()
	return err
}

func commissionDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
