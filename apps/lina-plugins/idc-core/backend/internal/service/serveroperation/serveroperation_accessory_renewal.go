// Accessory renewal shares one billing lifecycle for legacy and native server subjects.

package serveroperation

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

const (
	accessoryRenewalDefaultLead      = 7 * 24 * time.Hour
	accessoryRenewalRetry            = time.Hour
	PermissionAccessoryRenewalHandle = "idc:admin:resource:accessory-renewal:handle"
)

var errAccessoryRenewalNoLongerEligible = errors.New("server accessory renewal is no longer eligible")

type dueAccessoryRenewal struct {
	EntitlementNo, CustomerNo, ServiceNo, AccessoryKind, CurrencyCode string
	SubjectTypeCode, SubjectNo                                        string
	StatusCode, PendingInvoiceNo, PeriodUnitCode                      string
	CurrentPeriodEndAt                                                time.Time
	LastRenewalAttemptAt                                              *time.Time
}

func (w *workerImpl) reconcileAccessoryRenewals(ctx context.Context, limit int) (int, int, error) {
	rows := make([]dueAccessoryRenewal, 0, limit)
	if err := w.database.GetScan(ctx, &rows, `SELECT entitlement.entitlement_no,
entitlement.customer_no,COALESCE(entitlement.service_no,'') AS service_no,entitlement.accessory_kind,
entitlement.subject_type_code,entitlement.subject_no,
entitlement.currency_code,entitlement.status_code,
COALESCE(entitlement.pending_invoice_no,'') AS pending_invoice_no,
entitlement.period_unit_code,entitlement.current_period_end_at,
entitlement.last_renewal_attempt_at
FROM plugin_idc_core_server_accessory_entitlement entitlement
LEFT JOIN plugin_idc_core_service_instance service
 ON entitlement.subject_type_code='service' AND service.tenant_id=entitlement.tenant_id
 AND service.service_no=entitlement.service_no
LEFT JOIN plugin_idc_core_service_entitlement owner
 ON entitlement.subject_type_code='entitlement' AND owner.tenant_id=entitlement.tenant_id
 AND owner.entitlement_no=entitlement.subject_no AND owner.customer_no=entitlement.customer_no
 AND owner.currency_code=entitlement.currency_code AND owner.entitlement_owner_id='idc-resource'
LEFT JOIN plugin_idc_core_service_renewal_policy renewal_policy
 ON renewal_policy.tenant_id=entitlement.tenant_id
 AND renewal_policy.policy_no=COALESCE(owner.current_renewal_policy_no,service.current_renewal_policy_no)
 AND renewal_policy.policy_version=COALESCE(owner.current_renewal_policy_version,service.current_renewal_policy_version)
WHERE entitlement.tenant_id=0 AND entitlement.billing_mode_code='paid'
 AND (service.service_no IS NOT NULL OR owner.entitlement_no IS NOT NULL)
 AND entitlement.auto_renew_enabled AND entitlement.status_code IN ('active','payment_pending')
 AND entitlement.current_period_end_at IS NOT NULL
 AND entitlement.current_period_end_at <= CURRENT_TIMESTAMP +
     COALESCE(renewal_policy.lead_seconds, ?) * INTERVAL '1 second'
ORDER BY entitlement.current_period_end_at,entitlement.id LIMIT ?`,
		int64(accessoryRenewalDefaultLead/time.Second), limit); err != nil {
		return 0, 0, gerror.Wrap(err, "list due accessory renewals")
	}
	renewed, pending := 0, 0
	for index := range rows {
		outcome, err := w.reconcileAccessoryRenewal(ctx, &rows[index])
		if err != nil {
			return renewed, pending, err
		}
		switch outcome {
		case "renewed":
			renewed++
		case "payment_pending":
			pending++
		}
	}
	return renewed, pending, nil
}

func (w *workerImpl) reconcileAccessoryRenewal(ctx context.Context, row *dueAccessoryRenewal) (string, error) {
	if gdb.TXFromCtx(ctx, w.database.GetGroup()) != nil {
		return "", ErrOperationConflict
	}
	now, err := databaseNow(ctx, w.database)
	if err != nil {
		return "", err
	}
	if row.PendingInvoiceNo == "" {
		invoiceNo, total, createErr := w.createAccessoryRenewalInvoice(ctx, row, now)
		if errors.Is(createErr, errAccessoryRenewalNoLongerEligible) {
			return "skipped", nil
		}
		if createErr != nil {
			return "", createErr
		}
		row.PendingInvoiceNo = invoiceNo
		row.LastRenewalAttemptAt = nil
		if total == 0 {
			return "renewed", w.activatePaidAccessoryRenewal(ctx, row.EntitlementNo, invoiceNo)
		}
	}
	invoiceStatus, err := w.database.GetValue(ctx, `SELECT status_code
FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=?`, row.PendingInvoiceNo)
	if err != nil {
		return "", err
	}
	if invoiceStatus.String() == "paid" {
		return "renewed", w.activatePaidAccessoryRenewal(ctx, row.EntitlementNo, row.PendingInvoiceNo)
	}
	if row.LastRenewalAttemptAt == nil || !row.LastRenewalAttemptAt.Add(accessoryRenewalRetry).After(now) {
		if reminderErr := w.enqueueAccessoryRenewalReminder(ctx, row.EntitlementNo, row.PendingInvoiceNo); reminderErr != nil {
			if !row.CurrentPeriodEndAt.After(now) {
				if pendingErr := w.markAccessoryRenewalPaymentPending(ctx, row.EntitlementNo, row.PendingInvoiceNo, now); pendingErr != nil {
					return "", pendingErr
				}
			}
			return "", reminderErr
		}
		settled, payErr := w.payAccessoryRenewalFromBalance(ctx, row.EntitlementNo, row.PendingInvoiceNo, now)
		if payErr != nil && !bizerr.Is(payErr, billing.CodeInsufficientBalance) {
			return "", payErr
		}
		if settled {
			return "renewed", w.activatePaidAccessoryRenewal(ctx, row.EntitlementNo, row.PendingInvoiceNo)
		}
	}
	if !row.CurrentPeriodEndAt.After(now) {
		if err = w.markAccessoryRenewalPaymentPending(ctx, row.EntitlementNo, row.PendingInvoiceNo, now); err != nil {
			return "", err
		}
		return "payment_pending", nil
	}
	return "waiting_payment", nil
}

type accessoryRenewalPrice struct {
	PolicyNo, CurrencyCode, PeriodUnitCode, ContentDigest string
	NameZhCN, NameEnUS, TaxTreatmentCode, TaxCategoryCode string
	RoundingScopeCode, RoundingModeCode                   string
	PolicyVersion                                         int64
	UnitAmountMinor, RoundingQuantumMinor                 int64
	PeriodCount                                           int
	TaxRateBasisPoints                                    *int
}

func (w *workerImpl) createAccessoryRenewalInvoice(ctx context.Context, row *dueAccessoryRenewal,
	now time.Time) (string, int64, error) {
	var resultNo string
	var resultTotal int64
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked *struct {
			PendingInvoiceNo string     `orm:"pending_invoice_no"`
			CustomerNo       string     `orm:"customer_no"`
			StatusCode       string     `orm:"status_code"`
			AutoRenewEnabled bool       `orm:"auto_renew_enabled"`
			PeriodEnd        *time.Time `orm:"current_period_end_at"`
		}
		if err := w.database.GetScan(ctx, &locked, `SELECT COALESCE(pending_invoice_no,'') AS pending_invoice_no,
customer_no,status_code,auto_renew_enabled,current_period_end_at FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND entitlement_no=? FOR UPDATE`, row.EntitlementNo); err != nil {
			return err
		}
		if locked == nil || locked.PeriodEnd == nil || !locked.AutoRenewEnabled ||
			locked.CustomerNo != row.CustomerNo ||
			(locked.StatusCode != "active" && locked.StatusCode != "payment_pending") {
			return errAccessoryRenewalNoLongerEligible
		}
		if locked.PendingInvoiceNo != "" {
			resultNo = locked.PendingInvoiceNo
			value, err := w.database.GetValue(ctx, `SELECT total_amount_minor FROM plugin_idc_core_billing_invoice
WHERE tenant_id=0 AND invoice_no=?`, resultNo)
			resultTotal = value.Int64()
			return err
		}
		price, err := loadAccessoryRenewalPrice(ctx, w.database, row)
		if err != nil {
			return err
		}
		amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{
			PeriodUnitCode: price.PeriodUnitCode, PriceIncludesTax: true,
			TaxTreatmentCode: price.TaxTreatmentCode, TaxRateBasisPoints: price.TaxRateBasisPoints,
			TaxCategoryCode: price.TaxCategoryCode, RoundingScopeCode: price.RoundingScopeCode,
			RoundingModeCode: price.RoundingModeCode, RoundingQuantumMinor: price.RoundingQuantumMinor,
			Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1,
				ChargeAmountMinor: price.UnitAmountMinor, PricedAmountMinor: price.UnitAmountMinor}},
		}, 0)
		if err != nil || amounts.TotalAmountMinor != price.UnitAmountMinor {
			return ErrServiceUnavailable
		}
		periodKey := databaseTime(*locked.PeriodEnd).Format(time.RFC3339Nano)
		invoiceNo, _ := deterministicReference("ain_", row.EntitlementNo+":"+periodKey)
		itemNo, _ := deterministicReference("aii_", invoiceNo)
		dueAt := databaseTime(*locked.PeriodEnd)
		if !dueAt.After(now) {
			dueAt = now.Add(24 * time.Hour)
		}
		invoiceStatus := "pending_payment"
		var settledAt any
		if amounts.TotalAmountMinor == 0 {
			invoiceStatus, settledAt = "paid", now
		}
		invoiceDigest, err := digestValue(struct {
			InvoiceNo, EntitlementNo, PolicyNo, Currency string
			PolicyVersion, Total                         int64
			PeriodUnit                                   string
			PeriodCount                                  int
		}{invoiceNo, row.EntitlementNo, price.PolicyNo, price.CurrencyCode,
			price.PolicyVersion, amounts.TotalAmountMinor, price.PeriodUnitCode, price.PeriodCount})
		if err != nil {
			return err
		}
		itemDigest, err := digestValue(struct{ ItemNo, InvoiceNo, EntitlementNo, Digest string }{
			itemNo, invoiceNo, row.EntitlementNo, invoiceDigest})
		if err != nil {
			return err
		}
		if _, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice
(tenant_id,invoice_no,customer_no,source_code,currency_code,status_code,item_count,
subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,paid_amount_minor,
refundable_amount_minor,refunded_amount_minor,source_snapshot_digest,content_digest,payment_due_at,
issued_at,settled_at,version,created_at,updated_at)
VALUES (0,?,?,'renewal',?,?,1,?,0,?,?,?,?,0,0,?,?,?,?,?,1,?,?)`, invoiceNo,
			row.CustomerNo, price.CurrencyCode, invoiceStatus, amounts.SubtotalMinor, amounts.NetAmountMinor,
			amounts.TaxAmountMinor, amounts.TotalAmountMinor, int64(0),
			price.ContentDigest, invoiceDigest, dueAt, now, settledAt, now, now); err != nil {
			return gerror.Wrap(err, "insert accessory renewal invoice")
		}
		if _, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice_item
(tenant_id,invoice_item_no,invoice_no,customer_no,currency_code,line_number,item_type_code,
source_reference,description_zh_cn,description_en_us,subtotal_minor,discount_minor,net_amount_minor,
tax_amount_minor,total_amount_minor,refundable_cap_minor,content_digest)
VALUES (0,?,?,?,?,1,'renewal',?,?,?,?,0,?,?,?,0,?)`, itemNo, invoiceNo, row.CustomerNo,
			price.CurrencyCode, row.EntitlementNo, price.NameZhCN, price.NameEnUS,
			amounts.SubtotalMinor, amounts.NetAmountMinor, amounts.TaxAmountMinor,
			amounts.TotalAmountMinor, itemDigest); err != nil {
			return gerror.Wrap(err, "insert accessory renewal invoice item")
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET pending_invoice_no=?,pending_policy_no=?,pending_policy_version=?,pending_period_unit_code=?,
pending_period_count=?,pending_total_amount_minor=?,last_renewal_attempt_at=NULL,
version=version+1,updated_at=? WHERE tenant_id=0 AND entitlement_no=? AND pending_invoice_no IS NULL`,
			invoiceNo, price.PolicyNo, price.PolicyVersion, price.PeriodUnitCode, price.PeriodCount,
			amounts.TotalAmountMinor, now, row.EntitlementNo); err != nil {
			return err
		}
		resultNo, resultTotal = invoiceNo, amounts.TotalAmountMinor
		return nil
	})
	return resultNo, resultTotal, err
}

func loadAccessoryRenewalPrice(ctx context.Context, database gdb.DB,
	row *dueAccessoryRenewal) (*accessoryRenewalPrice, error) {
	if row.SubjectTypeCode == nativeAccessorySubject {
		return loadNativeAccessoryRenewalPrice(ctx, database, row)
	}
	var price *accessoryRenewalPrice
	if err := database.GetScan(ctx, &price, `SELECT policy.policy_no,version.policy_version,
version.currency_code,version.unit_amount_minor,version.period_unit_code,version.period_count,
version.content_digest,kind.name_zh_cn,kind.name_en_us,service_price.tax_treatment_code,
service_price.tax_rate_basis_points,service_price.tax_category_code,service_price.rounding_scope_code,
service_price.rounding_mode_code,service_price.rounding_quantum_minor
FROM plugin_idc_core_server_accessory_price_policy policy
JOIN plugin_idc_core_server_accessory_price_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
JOIN plugin_idc_core_server_accessory_kind kind ON kind.accessory_kind=version.accessory_kind
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id=0 AND service.service_no=? AND service.customer_no=?
JOIN plugin_idc_core_service_price_snapshot service_price
 ON service_price.tenant_id=service.tenant_id
 AND service_price.price_snapshot_no=service.current_price_snapshot_no
WHERE policy.tenant_id=0 AND policy.accessory_kind=? AND policy.currency_code=?`,
		row.ServiceNo, row.CustomerNo, row.AccessoryKind, row.CurrencyCode); err != nil {
		return nil, err
	}
	if price == nil {
		return nil, ErrServiceUnavailable
	}
	return price, nil
}

func (w *workerImpl) payAccessoryRenewalFromBalance(ctx context.Context, entitlementNo, invoiceNo string,
	now time.Time) (bool, error) {
	digest, err := digestValue(struct{ EntitlementNo, InvoiceNo string }{entitlementNo, invoiceNo})
	if err != nil {
		return false, err
	}
	_, err = w.invoices.PayFromBalance(ctx, billing.InvoiceBalancePaymentInput{
		InvoiceNo: invoiceNo, SourceReference: "accessory-renewal:" + invoiceNo,
		SourceSnapshotDigest: digest,
	})
	_, updateErr := w.database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_entitlement
SET last_renewal_attempt_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND pending_invoice_no=?`, now, now, entitlementNo, invoiceNo)
	if updateErr != nil {
		return false, updateErr
	}
	return err == nil, err
}

func (w *workerImpl) activatePaidAccessoryRenewal(ctx context.Context, entitlementNo, invoiceNo string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row *struct {
			StatusCode         string    `orm:"status_code"`
			PendingInvoiceNo   string    `orm:"pending_invoice_no"`
			PendingPeriodUnit  string    `orm:"pending_period_unit_code"`
			PendingPeriodCount int       `orm:"pending_period_count"`
			CurrentPeriodEndAt time.Time `orm:"current_period_end_at"`
		}
		if err := w.database.GetScan(ctx, &row, `SELECT status_code,COALESCE(pending_invoice_no,'') AS pending_invoice_no,
pending_period_unit_code,pending_period_count,current_period_end_at
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND entitlement_no=? FOR UPDATE`, entitlementNo); err != nil {
			return err
		}
		if row == nil || row.PendingInvoiceNo != invoiceNo {
			return ErrOperationConflict
		}
		var invoice *struct {
			Paid      bool
			SettledAt *time.Time
		}
		err := w.database.GetScan(ctx, &invoice, `SELECT status_code='paid' AS paid,settled_at
FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=? FOR UPDATE`, invoiceNo)
		if err != nil || invoice == nil || !invoice.Paid || invoice.SettledAt == nil || invoice.SettledAt.IsZero() {
			return ErrOperationConflict
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		start := databaseTime(row.CurrentPeriodEndAt)
		settledAt := databaseTime(*invoice.SettledAt)
		if !start.After(settledAt) {
			start = settledAt
		}
		end, ok := addAccessoryPeriod(start, row.PendingPeriodUnit, row.PendingPeriodCount)
		if !ok {
			return ErrInvalidInput
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='active',current_period_start_at=?,current_period_end_at=?,pending_invoice_no=NULL,
pending_policy_no=NULL,pending_policy_version=NULL,pending_period_unit_code=NULL,pending_period_count=NULL,
pending_total_amount_minor=NULL,last_renewal_attempt_at=NULL,version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND pending_invoice_no=?`, start, end, now, entitlementNo, invoiceNo); err != nil {
			return err
		}
		_, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_renewal_work_item
SET status_code='resolved',resolved_at=?,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND invoice_no=? AND status_code='open'`, now, now, entitlementNo, invoiceNo)
		return err
	})
}

func (w *workerImpl) markAccessoryRenewalPaymentPending(ctx context.Context, entitlementNo, invoiceNo string,
	now time.Time) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='payment_pending',version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND pending_invoice_no=? AND status_code='active'`, now,
			entitlementNo, invoiceNo)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			return err
		}
		var owner *struct{ CustomerNo, ServiceNo, SubjectTypeCode, SubjectNo string }
		if err = w.database.GetScan(ctx, &owner, `SELECT customer_no,COALESCE(service_no,'') AS service_no,subject_type_code,subject_no
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND entitlement_no=?`, entitlementNo); err != nil || owner == nil {
			return err
		}
		workNo, _ := deterministicReference("arw_", entitlementNo+":"+invoiceNo)
		_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_renewal_work_item
(tenant_id,work_item_no,entitlement_no,customer_no,service_no,subject_type_code,subject_no,invoice_no,required_permission,
status_code,reason_code,created_at,updated_at)
VALUES (0,?,?,?,?,?,?,?,?,'open','balance_payment_failed',?,?) ON CONFLICT (invoice_no) DO NOTHING`,
			workNo, entitlementNo, owner.CustomerNo, nullableAccessoryServiceNo(owner.ServiceNo), owner.SubjectTypeCode, owner.SubjectNo, invoiceNo,
			PermissionAccessoryRenewalHandle, now, now)
		if err != nil {
			return err
		}
		return w.enqueueAccessoryRenewalCustomerNotification(ctx, owner.CustomerNo, invoiceNo)
	})
}

func (w *workerImpl) enqueueAccessoryRenewalCustomerNotification(ctx context.Context,
	customerNo, invoiceNo string) error {
	var contacts []struct {
		ContactNo string `orm:"contact_no"`
	}
	if err := w.database.GetScan(ctx, &contacts, `SELECT contact.contact_no
FROM plugin_idc_core_customer_contact_role role
JOIN plugin_idc_core_customer_contact contact
 ON contact.tenant_id=role.tenant_id AND contact.customer_no=role.customer_no
 AND contact.contact_no=role.contact_no
WHERE role.tenant_id=0 AND role.customer_no=? AND role.role_code='billing'
 AND role.status_code='active' AND contact.status_code='active'
ORDER BY contact.id LIMIT 2`, customerNo); err != nil {
		return err
	}
	if len(contacts) != 1 {
		return nil
	}
	versionValue, err := w.database.GetValue(ctx, `SELECT version
FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=?`, invoiceNo)
	if err != nil {
		return err
	}
	_, err = w.notifications.Enqueue(ctx, notificationcap.EventRequest{
		EventCode: accessoryRenewalFailureEvent,
		Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact,
			CustomerNo: customerNo, ContactNo: contacts[0].ContactNo},
		BusinessObjectType: notificationcap.BusinessObjectBilling, BusinessObjectNo: invoiceNo,
		BusinessObjectVersion: versionValue.Int64(),
		Variables:             map[string]string{"invoice_no": invoiceNo},
		CorrelationNo:         invoiceNo, IdempotencyKey: "accessory-renewal-payment-pending:" + invoiceNo,
	})
	return err
}
