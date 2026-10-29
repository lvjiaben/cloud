// Native deletion closes an unstarted renewal bill through billing's public
// owner before releasing object occupancy. Uncertain money state remains linked.

package serveroperation

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// nativeClosePendingRenewalInvoice returns pending=true only after persisting a
// non-renewing, proven-deleted object with an unresolved invoice association.
// The caller commits that state before returning a recoverable error. A false
// result permits nativeCompleteDeletedAccessory in this same transaction.
func nativeClosePendingRenewalInvoice(ctx context.Context, database gdb.DB, invoices billing.InvoiceService,
	object servercommercialcap.ObjectRequest, completion servercommercialcap.Completion) (bool, error) {
	if invoices == nil || gdb.TXFromCtx(ctx, database.GetGroup()) == nil ||
		completion.Status != servercommercialcap.CompletionSucceeded || completion.EntitlementNo != object.EntitlementNo ||
		completion.ObjectNo != object.ObjectNo || !validReference(completion.OperationNo, 40) ||
		!validNativeLifecycleDigest(completion.EvidenceDigest) || completion.ObservedAt.IsZero() {
		return false, ErrOperationConflict
	}
	var row *struct {
		EntitlementNo, PendingInvoiceNo, CurrencyCode string
		StatusCode                                    nativeAccessoryState
	}
	if err := database.GetScan(ctx, &row, `SELECT entitlement_no,COALESCE(pending_invoice_no,'') AS pending_invoice_no,currency_code,status_code
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=? AND customer_no=?
 AND accessory_kind=? AND object_no=? FOR UPDATE`, object.EntitlementNo, object.CustomerNo, object.Kind, object.ObjectNo); err != nil {
		return false, err
	}
	if row == nil || row.PendingInvoiceNo == "" {
		return false, nil
	}
	var invoice *struct{ StatusCode string }
	if err := database.GetScan(ctx, &invoice, `SELECT status_code FROM plugin_idc_core_billing_invoice
WHERE tenant_id=0 AND invoice_no=? AND customer_no=? AND currency_code=? FOR UPDATE`,
		row.PendingInvoiceNo, object.CustomerNo, row.CurrencyCode); err != nil {
		return false, err
	}
	if invoice == nil {
		return true, nativeMarkDeletionPending(ctx, database, row.EntitlementNo, row.StatusCode, row.PendingInvoiceNo, object, completion)
	}
	if invoice.StatusCode == string(billing.InvoiceStatusPaid) || invoice.StatusCode == string(billing.InvoiceStatusCancelled) {
		return false, nil
	}
	closed, err := invoices.CancelUnpaid(ctx, row.PendingInvoiceNo)
	if err != nil {
		// Billing's nested transaction rolls back its failed attempt before
		// this source-owned pending transition is committed by the caller.
		return true, nativeMarkDeletionPending(ctx, database, row.EntitlementNo, row.StatusCode, row.PendingInvoiceNo, object, completion)
	}
	if closed == nil || closed.InvoiceNo != row.PendingInvoiceNo || closed.CustomerNo != object.CustomerNo ||
		closed.CurrencyCode != row.CurrencyCode || closed.Status != billing.InvoiceStatusCancelled {
		return true, nativeMarkDeletionPending(ctx, database, row.EntitlementNo, row.StatusCode, row.PendingInvoiceNo, object, completion)
	}
	return false, nil
}

// nativeMarkDeletionPending records the verified resource deletion while money
// remains unresolved. It never detaches the bill, retries payment or releases an unknown result.
func nativeMarkDeletionPending(ctx context.Context, database gdb.DB, entitlementNo string, from nativeAccessoryState,
	invoiceNo string, object servercommercialcap.ObjectRequest, completion servercommercialcap.Completion) error {
	if from != nativeAccessoryDeleting {
		if err := execOne(ctx, database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='deletion_pending',auto_renew_enabled=FALSE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND status_code=? AND pending_invoice_no=?`, entitlementNo, from, invoiceNo); err != nil {
			return err
		}
	}
	workNo, err := deterministicReference("arw_", entitlementNo+":"+invoiceNo)
	if err != nil {
		return err
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_renewal_work_item
(tenant_id,work_item_no,entitlement_no,customer_no,subject_type_code,subject_no,invoice_no,required_permission,status_code,reason_code,created_at,updated_at)
VALUES(0,?,?,?,'entitlement',?,?,?,'open','deletion_invoice_pending',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(invoice_no) DO NOTHING`, workNo, entitlementNo, object.CustomerNo, object.EntitlementNo, invoiceNo, PermissionAccessoryRenewalHandle); err != nil {
		return err
	}
	if from == nativeAccessoryDeleting {
		return nil
	}
	eventNo, err := deterministicReference("aee_", entitlementNo+":deletion-pending:"+completion.OperationNo)
	if err != nil {
		return err
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,operation_no,invoice_no,evidence_digest,occurred_at)
VALUES(0,?,?,'deletion_pending',?,'deletion_pending',?,?,?,?) ON CONFLICT(event_no) DO NOTHING`, eventNo,
		entitlementNo, from, completion.OperationNo, invoiceNo, completion.EvidenceDigest, databaseTime(completion.ObservedAt))
	return err
}
