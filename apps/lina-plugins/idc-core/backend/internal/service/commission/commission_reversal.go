// This file reverses cumulative actual refunds, preserving ledger and accrual
// conservation even when refund notifications arrive before the accrual event.
package commission

import (
	"context"
	"math/big"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

func (c *consumer) consumeRefund(ctx context.Context, event outbox.ClaimedEvent) error {
	payload, digest, err := decodeRefundEvent(event)
	if err != nil {
		return err
	}
	return c.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var invoice *entity.BillingInvoice
		if err := c.database.Model(dao.BillingInvoice.Table()).Ctx(ctx).Where(do.BillingInvoice{TenantId: 0, InvoiceNo: payload.InvoiceNo}).LockUpdate().Scan(&invoice); err != nil {
			return err
		}
		if invoice == nil || invoice.CustomerNo != payload.CustomerNo || invoice.CurrencyCode != payload.CurrencyCode {
			return permanent("commission_refund_invoice_mismatch")
		}
		var receipt *entity.CommissionRefundReceipt
		if err := c.database.Model(dao.CommissionRefundReceipt.Table()).Ctx(ctx).Where(do.CommissionRefundReceipt{RefundNo: payload.RefundNo}).Scan(&receipt); err != nil {
			return err
		}
		if receipt != nil {
			if receipt.PayloadDigest != digest {
				return permanent("commission_refund_replay_conflict")
			}
			return nil
		}
		// The successful source command must belong to this invoice; payload alone is never financial evidence.
		count, err := c.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation a JOIN plugin_idc_core_refund_execution_command c ON c.command_no=a.refund_source_reference WHERE c.refund_no=? AND a.invoice_no=? AND a.direction_code='apply'`, payload.RefundNo, payload.InvoiceNo)
		if err != nil {
			return err
		}
		if count.Int() == 0 {
			return permanent("commission_refund_evidence_missing")
		}
		if err = c.accrueLockedInvoice(ctx, invoice, ""); err != nil {
			return err
		}
		var accruals []entity.CommissionAccrual
		if err = c.database.Model(dao.CommissionAccrual.Table()).Ctx(ctx).Where(do.CommissionAccrual{TenantId: 0, InvoiceNo: invoice.InvoiceNo}).OrderAsc("beneficiary_type_code").OrderAsc("beneficiary_reference").OrderAsc("id").LockUpdate().Scan(&accruals); err != nil {
			return err
		}
		var amounts []struct {
			InvoiceItemNo string `orm:"invoice_item_no"`
			Amount        int64  `orm:"amount_minor"`
		}
		if err = c.database.GetScan(ctx, &amounts, `SELECT invoice_item_no,SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END) AS amount_minor FROM plugin_idc_core_billing_refund_allocation WHERE tenant_id=0 AND invoice_no=? GROUP BY invoice_item_no`, invoice.InvoiceNo); err != nil {
			return err
		}
		refunded := map[string]int64{}
		for _, amount := range amounts {
			refunded[amount.InvoiceItemNo] = amount.Amount
		}
		var previous []struct {
			CommissionNo string `orm:"commission_no"`
			Amount       int64  `orm:"amount_minor"`
		}
		if err = c.database.GetScan(ctx, &previous, `SELECT r.commission_no,SUM(r.amount_minor) AS amount_minor FROM plugin_idc_core_commission_refund_reversal r JOIN plugin_idc_core_commission_accrual a ON a.commission_no=r.commission_no WHERE a.invoice_no=? GROUP BY r.commission_no`, invoice.InvoiceNo); err != nil {
			return err
		}
		reversedBefore := map[string]int64{}
		for _, row := range previous {
			reversedBefore[row.CommissionNo] = row.Amount
		}
		var summary reversalSummary
		for _, accrual := range accruals {
			amount, err := proportionalReversal(accrual.OriginalAmountMinor, refunded[accrual.InvoiceItemNo], accrual.InvoiceItemAmountMinor, reversedBefore[accrual.CommissionNo])
			if err != nil {
				return err
			}
			if amount == 0 {
				continue
			}
			if err = c.reverseAccrual(ctx, event.EventNo, payload.RefundNo, accrual, amount); err != nil {
				return err
			}
			summary.Count++
			summary.Amount += amount
		}
		_, err = c.database.Model(dao.CommissionRefundReceipt.Table()).Ctx(ctx).Data(do.CommissionRefundReceipt{TenantId: 0, EventNo: event.EventNo, RefundNo: payload.RefundNo, PayloadDigest: digest, ReversalCount: summary.Count, ReversedAmountMinor: summary.Amount}).Insert()
		return err
	})
}

func (c *consumer) reverseAccrual(ctx context.Context, eventNo, refundNo string, accrual entity.CommissionAccrual, amount int64) error {
	account, err := c.lockAccount(ctx, beneficiary{Kind: accrual.BeneficiaryTypeCode, Reference: accrual.BeneficiaryReference}, accrual.CurrencyCode, false)
	if err != nil {
		return err
	}
	var allocations struct {
		Paid   int64 `orm:"paid"`
		Frozen int64 `orm:"frozen"`
	}
	if err = c.database.GetScan(ctx, &allocations, `SELECT
 COALESCE((SELECT SUM(amount_minor) FROM plugin_idc_core_sales_commission_offline_payout_item WHERE commission_no=?),0)+
 COALESCE((SELECT SUM(i.amount_minor) FROM plugin_idc_core_sales_commission_settlement_item i JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no WHERE i.commission_no=? AND s.status_code='paid'),0) AS paid,
 COALESCE((SELECT SUM(i.amount_minor) FROM plugin_idc_core_sales_commission_settlement_item i JOIN plugin_idc_core_sales_commission_settlement s ON s.settlement_no=i.settlement_no WHERE i.commission_no=? AND s.status_code IN ('pending_review','approved','batched','manual_review')),0) AS frozen`, accrual.CommissionNo, accrual.CommissionNo, accrual.CommissionNo); err != nil {
		return err
	}
	priorDebt, err := c.database.GetValue(ctx, `SELECT COALESCE(SUM(amount_minor),0) FROM plugin_idc_core_sales_commission_recovery WHERE commission_no=?`, accrual.CommissionNo)
	if err != nil {
		return err
	}
	var (
		pending          = min(amount, accrual.PendingAmountMinor)
		unpaid           = max(int64(0), accrual.MaturedAmountMinor-max(int64(0), allocations.Paid-priorDebt.Int64())-allocations.Frozen)
		mature           = min(amount-pending, unpaid)
		debt             = amount - pending - mature
		projectionAmount = min(amount, accrual.PendingAmountMinor+accrual.MaturedAmountMinor)
		projectionMature = projectionAmount - pending
	)

	if allocations.Frozen > 0 && debt > 0 {
		return retryable(gerror.New("commission refund awaits release of reserved settlement"))
	}
	if pending > account.PendingAmountMinor || mature > account.AvailableAmountMinor {
		return retryable(gerror.New("commission reversal balance is reserved"))
	}
	number := "CRV_" + commissionDigest(accrual.CommissionNo + ":" + refundNo)[:32]
	if _, err = c.database.Model(dao.CommissionRefundReversal.Table()).Ctx(ctx).Data(do.CommissionRefundReversal{TenantId: 0, ReversalNo: number, CommissionNo: accrual.CommissionNo, RefundNo: refundNo, SourceEventNo: eventNo, AmountMinor: amount}).Insert(); err != nil {
		return err
	}
	if pending+mature > 0 {
		if err = c.appendAccountEntry(ctx, account, entryReversal, sourceReversal, number, 0, -pending, -mature, 0, pending+mature); err != nil {
			return err
		}
	}
	if debt > 0 {
		_, err = c.database.Model(dao.SalesCommissionRecovery.Table()).Ctx(ctx).Data(do.SalesCommissionRecovery{TenantId: 0, RecoveryNo: "CRE_" + commissionDigest(number)[:32], AccountNo: account.AccountNo, CommissionNo: accrual.CommissionNo, RefundNo: refundNo, ReversalNo: number, AmountMinor: debt}).Insert()
		if err != nil {
			return err
		}
	}
	status := statusPartiallyReversed
	if accrual.PendingAmountMinor-pending == 0 {
		status = statusMatured
		if accrual.MaturedAmountMinor-projectionMature == 0 {
			status = statusReversed
		}
	}
	_, err = c.database.Model(dao.CommissionAccrual.Table()).Ctx(ctx).Where(do.CommissionAccrual{CommissionNo: accrual.CommissionNo}).Data(do.CommissionAccrual{PendingAmountMinor: accrual.PendingAmountMinor - pending, MaturedAmountMinor: accrual.MaturedAmountMinor - projectionMature, ReversedAmountMinor: accrual.ReversedAmountMinor + projectionAmount, StatusCode: status}).Update()
	return err
}

// proportionalReversal uses the cumulative refund, not the latest notification's amount.
func proportionalReversal(original, refunded, total, already int64) (int64, error) {
	if original < 0 || total <= 0 || refunded < 0 || already < 0 || already > original {
		return 0, permanent("commission_reversal_amount_invalid")
	}
	refunded = min(refunded, total)
	amount := new(big.Int).Mul(big.NewInt(original), big.NewInt(refunded))
	amount.Quo(amount, big.NewInt(total))
	if !amount.IsInt64() {
		return 0, permanent("commission_reversal_amount_overflow")
	}
	return max(int64(0), amount.Int64()-already), nil
}
