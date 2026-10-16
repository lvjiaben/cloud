// This file appends immutable commission ledger entries under the account lock.
// Database triggers own all balance updates; version is not an entry sequence.
package commission

import (
	"context"
	"strconv"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (c *consumer) lockAccount(ctx context.Context, owner beneficiary, currency string, create bool) (*entity.SalesCommissionAccount, error) {
	number := "CAC_" + commissionDigest(owner.Kind + ":" + owner.Reference + ":" + currency)[:32]
	if create {
		// A unique owner/currency key safely handles simultaneous accrual on different invoices.
		if _, err := c.database.Exec(ctx, `INSERT INTO plugin_idc_core_sales_commission_account
 (tenant_id,account_no,beneficiary_type_code,beneficiary_reference,currency_code,status_code)
 VALUES (0,?,?,?,?,?) ON CONFLICT DO NOTHING`, number, owner.Kind, owner.Reference, currency, accountActive); err != nil {
			return nil, err
		}
	}
	var account *entity.SalesCommissionAccount
	err := c.database.Model(dao.SalesCommissionAccount.Table()).Ctx(ctx).Where(do.SalesCommissionAccount{TenantId: 0, BeneficiaryTypeCode: owner.Kind, BeneficiaryReference: owner.Reference, CurrencyCode: currency}).LockUpdate().Scan(&account)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, permanent("commission_account_missing")
	}
	return account, nil
}

func (c *consumer) appendAccountEntry(ctx context.Context, account *entity.SalesCommissionAccount, kind, source, reference string, credited, pending, available, paid, reversed int64) error {
	sequence, err := c.database.GetValue(ctx, `SELECT COALESCE(MAX(entry_sequence),0)+1 FROM plugin_idc_core_sales_commission_account_entry WHERE account_no=?`, account.AccountNo)
	if err != nil {
		return err
	}
	digest := commissionDigest(source + ":" + reference + ":" + strconv.FormatInt(credited, 10) + ":" + strconv.FormatInt(pending, 10) + ":" + strconv.FormatInt(available, 10) + ":" + strconv.FormatInt(paid, 10) + ":" + strconv.FormatInt(reversed, 10))
	data := do.SalesCommissionAccountEntry{TenantId: 0, EntryNo: "CEN_" + digest[:32], AccountNo: account.AccountNo, CurrencyCode: account.CurrencyCode, EntrySequence: sequence.Int64(), EntryTypeCode: kind, SourceTypeCode: source, SourceReference: reference, SourceDigest: digest, DeltaCreditedMinor: credited, DeltaPendingMinor: pending, DeltaAvailableMinor: available, DeltaFrozenMinor: 0, DeltaPaidMinor: paid, DeltaReversedMinor: reversed, AfterCreditedMinor: account.CreditedAmountMinor + credited, AfterPendingMinor: account.PendingAmountMinor + pending, AfterAvailableMinor: account.AvailableAmountMinor + available, AfterFrozenMinor: account.FrozenAmountMinor, AfterPaidMinor: account.PaidAmountMinor + paid, AfterReversedMinor: account.ReversedAmountMinor + reversed}
	_, err = c.database.Model(dao.SalesCommissionAccountEntry.Table()).Ctx(ctx).Data(data).Insert()
	return err
}
