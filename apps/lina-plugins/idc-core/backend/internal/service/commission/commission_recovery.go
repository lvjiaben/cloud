// This file offsets unpaid mature commissions against prior paid-refund debt.
// Historical cash payouts remain unchanged; each offset has its own ledger evidence.
package commission

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const sourceRecovery = "commission_recovery"

func (c *consumer) recoverAvailable(ctx context.Context, limit int) (int, error) {
	var accounts []entity.SalesCommissionAccount
	err := c.database.Model(dao.SalesCommissionAccount.Table()).Ctx(ctx).Where(do.SalesCommissionAccount{TenantId: 0}).WhereGT("available_amount_minor", 0).Where(`EXISTS (SELECT 1 FROM plugin_idc_core_sales_commission_recovery r WHERE r.account_no=plugin_idc_core_sales_commission_account.account_no AND r.amount_minor>(SELECT COALESCE(SUM(o.amount_minor),0) FROM plugin_idc_core_sales_commission_recovery_offset o WHERE o.recovery_no=r.recovery_no))`).OrderAsc("id").Limit(limit).Scan(&accounts)
	if err != nil {
		return 0, err
	}
	for i, account := range accounts {
		if err = c.recoverAccount(ctx, account); err != nil {
			return i, err
		}
	}
	return len(accounts), nil
}

func (c *consumer) recoverAccount(ctx context.Context, candidate entity.SalesCommissionAccount) error {
	return c.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Accrual locks precede the account lock, matching maturity and refund lock order.
		var accruals []entity.CommissionAccrual
		if err := c.database.Model(dao.CommissionAccrual.Table()).Ctx(ctx).Where(do.CommissionAccrual{TenantId: 0, BeneficiaryTypeCode: candidate.BeneficiaryTypeCode, BeneficiaryReference: candidate.BeneficiaryReference, CurrencyCode: candidate.CurrencyCode}).WhereGT("matured_amount_minor", 0).Where("matured_amount_minor>COALESCE((SELECT SUM(p.amount_minor) FROM plugin_idc_core_sales_commission_offline_payout_item p WHERE p.commission_no=plugin_idc_core_commission_accrual.commission_no),0)+COALESCE((SELECT SUM(i.amount_minor) FROM plugin_idc_core_sales_commission_settlement_item i JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no WHERE i.commission_no=plugin_idc_core_commission_accrual.commission_no AND s.status_code IN ('pending_review','approved','batched','paid','manual_review')),0)").OrderAsc("id").Limit(maximumInvoiceItems).LockUpdate().Scan(&accruals); err != nil {
			return err
		}
		account, err := c.lockAccount(ctx, beneficiary{Kind: candidate.BeneficiaryTypeCode, Reference: candidate.BeneficiaryReference}, candidate.CurrencyCode, false)
		if err != nil {
			return err
		}
		if account.AvailableAmountMinor <= 0 {
			return nil
		}
		var debts []struct {
			RecoveryNo string `orm:"recovery_no"`
			Amount     int64  `orm:"amount_minor"`
		}
		if err = c.database.GetScan(ctx, &debts, `SELECT r.recovery_no,r.amount_minor-COALESCE(SUM(o.amount_minor),0) AS amount_minor FROM plugin_idc_core_sales_commission_recovery r LEFT JOIN plugin_idc_core_sales_commission_recovery_offset o ON o.recovery_no=r.recovery_no WHERE r.account_no=? GROUP BY r.id,r.recovery_no,r.amount_minor HAVING r.amount_minor>COALESCE(SUM(o.amount_minor),0) ORDER BY r.id LIMIT ?`, account.AccountNo, maximumInvoiceItems); err != nil {
			return err
		}
		var paidRows []struct {
			CommissionNo string `orm:"commission_no"`
			Amount       int64  `orm:"amount_minor"`
		}
		keys := make([]string, 0, len(accruals))
		for _, accrual := range accruals {
			keys = append(keys, accrual.CommissionNo)
		}
		if len(keys) == 0 {
			return nil
		}
		if err = c.database.Model(dao.SalesCommissionOfflinePayoutItem.Table()).Ctx(ctx).Fields("commission_no,SUM(amount_minor) AS amount_minor").WhereIn("commission_no", keys).Group("commission_no").Scan(&paidRows); err != nil {
			return err
		}

		var legacyRows []struct {
			CommissionNo string `orm:"commission_no"`
			Amount       int64  `orm:"amount_minor"`
		}
		if err = c.database.GetScan(ctx, &legacyRows, `SELECT i.commission_no,SUM(i.amount_minor) AS amount_minor FROM plugin_idc_core_sales_commission_settlement_item i JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no WHERE i.commission_no IN (?) AND s.status_code IN ('pending_review','approved','batched','paid','manual_review') GROUP BY i.commission_no`, keys); err != nil {
			return err
		}
		paidRows = append(paidRows, legacyRows...)
		paid := map[string]int64{}
		for _, row := range paidRows {
			paid[row.CommissionNo] += row.Amount
		}
		for _, accrual := range accruals {
			available := min(account.AvailableAmountMinor, max(int64(0), accrual.MaturedAmountMinor-paid[accrual.CommissionNo]))
			if available == 0 {
				continue
			}
			for i := range debts {
				amount := min(available, debts[i].Amount)
				if amount == 0 {
					continue
				}
				ref := "COF_" + commissionDigest(fmt.Sprintf("%s:%s:%d", debts[i].RecoveryNo, accrual.CommissionNo, accrual.ReversedAmountMinor))[:32]
				if err = c.appendAccountEntry(ctx, account, entryReversal, sourceRecovery, ref, 0, 0, -amount, 0, amount); err != nil {
					return err
				}
				var entry entity.SalesCommissionAccountEntry
				if err = c.database.Model(dao.SalesCommissionAccountEntry.Table()).Ctx(ctx).Where(do.SalesCommissionAccountEntry{AccountNo: account.AccountNo, SourceTypeCode: sourceRecovery, SourceReference: ref}).Scan(&entry); err != nil {
					return err
				}
				if _, err = c.database.Model(dao.SalesCommissionRecoveryOffset.Table()).Ctx(ctx).Data(do.SalesCommissionRecoveryOffset{TenantId: 0, OffsetNo: ref, RecoveryNo: debts[i].RecoveryNo, CommissionNo: accrual.CommissionNo, AccountNo: account.AccountNo, AmountMinor: amount, EntryNo: entry.EntryNo}).Insert(); err != nil {
					return err
				}
				accrual.MaturedAmountMinor -= amount
				accrual.ReversedAmountMinor += amount
				account.AvailableAmountMinor -= amount
				account.ReversedAmountMinor += amount
				available -= amount
				debts[i].Amount -= amount
			}
			status := statusMatured
			if accrual.MaturedAmountMinor == 0 {
				status = statusReversed
			}
			if _, err = c.database.Model(dao.CommissionAccrual.Table()).Ctx(ctx).Where(do.CommissionAccrual{CommissionNo: accrual.CommissionNo}).Data(do.CommissionAccrual{MaturedAmountMinor: accrual.MaturedAmountMinor, ReversedAmountMinor: accrual.ReversedAmountMinor, StatusCode: status}).Update(); err != nil {
				return err
			}
		}
		return nil
	})
}
