// Accessory renewal reminders commit the frozen payable amount to the existing
// notification outbox before any automatic balance attempt is permitted.

package serveroperation

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"golang.org/x/text/currency"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

// accessoryRenewalReminderEvent is registered by the notification owner's matching migration.
const accessoryRenewalReminderEvent = "billing.accessory_renewal_due"

const accessoryRenewalFailureEvent = "billing.accessory_renewal_failed"

// enqueueAccessoryRenewalReminder retries the same notification intent, not a
// provider send. Missing/ambiguous billing contacts and failed enqueue stop debit.
func (w *workerImpl) enqueueAccessoryRenewalReminder(ctx context.Context, entitlementNo, invoiceNo string) error {
	if w.notifications == nil || gdb.TXFromCtx(ctx, w.database.GetGroup()) != nil {
		return ErrServiceUnavailable
	}
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var bill *struct {
			CustomerNo, CurrencyCode, PeriodUnit string
			Amount, TaxAmount, Version           int64
			PeriodCount                          int
		}
		if err := w.database.GetScan(ctx, &bill, `SELECT invoice.customer_no,invoice.currency_code,
invoice.total_amount_minor AS amount,invoice.tax_amount_minor AS tax_amount,invoice.version,
accessory.pending_period_unit_code AS period_unit,accessory.pending_period_count AS period_count
FROM plugin_idc_core_server_accessory_entitlement accessory
JOIN plugin_idc_core_billing_invoice invoice
 ON invoice.tenant_id=accessory.tenant_id AND invoice.invoice_no=accessory.pending_invoice_no
 AND invoice.customer_no=accessory.customer_no AND invoice.currency_code=accessory.currency_code
WHERE accessory.tenant_id=0 AND accessory.entitlement_no=? AND accessory.pending_invoice_no=?
 AND accessory.auto_renew_enabled AND accessory.status_code IN ('active','payment_pending')
 AND invoice.total_amount_minor=accessory.pending_total_amount_minor
 AND invoice.status_code IN ('pending_payment','overdue') FOR UPDATE OF accessory,invoice`, entitlementNo, invoiceNo); err != nil {
			return err
		}
		if bill == nil {
			return ErrOperationConflict
		}
		var contacts []struct{ ContactNo string }
		if err := w.database.GetScan(ctx, &contacts, `SELECT contact.contact_no
FROM plugin_idc_core_customer_contact_role role
JOIN plugin_idc_core_customer_contact contact
 ON contact.tenant_id=role.tenant_id AND contact.customer_no=role.customer_no AND contact.contact_no=role.contact_no
WHERE role.tenant_id=0 AND role.customer_no=? AND role.role_code='billing'
 AND role.status_code='active' AND contact.status_code='active'
ORDER BY contact.id LIMIT 2`, bill.CustomerNo); err != nil {
			return err
		}
		if len(contacts) != 1 {
			return ErrServiceUnavailable
		}
		amount, err := formatAccessoryRenewalAmount(bill.Amount, bill.CurrencyCode)
		if err != nil {
			return err
		}
		taxAmount, err := formatAccessoryRenewalAmount(bill.TaxAmount, bill.CurrencyCode)
		if err != nil {
			return err
		}
		_, err = w.notifications.Enqueue(ctx, notificationcap.EventRequest{
			EventCode: accessoryRenewalReminderEvent,
			Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact,
				CustomerNo: bill.CustomerNo, ContactNo: contacts[0].ContactNo},
			BusinessObjectType: notificationcap.BusinessObjectBilling, BusinessObjectNo: invoiceNo,
			BusinessObjectVersion: bill.Version,
			Variables: map[string]string{"invoice_no": invoiceNo, "amount": amount, "currency": bill.CurrencyCode,
				"tax_amount": taxAmount, "period_unit": bill.PeriodUnit, "period_count": strconv.Itoa(bill.PeriodCount)},
			CorrelationNo: invoiceNo, IdempotencyKey: "accessory-renewal-before-payment:" + invoiceNo,
		})
		return err
	})
}

// formatAccessoryRenewalAmount applies the ISO accounting minor-unit exponent
// with integer arithmetic; JPY and three-decimal currencies cannot be divided by 100.
func formatAccessoryRenewalAmount(minor int64, code string) (string, error) {
	unit, err := currency.ParseISO(code)
	if err != nil || unit.String() != code || minor < 0 {
		return "", ErrInvalidInput
	}
	precision, _ := currency.Standard.Rounding(unit)
	if precision == 0 {
		return strconv.FormatInt(minor, 10), nil
	}
	if precision < 0 || precision > 9 {
		return "", ErrInvalidInput
	}
	divisor := int64(1)
	for range precision {
		divisor *= 10
	}
	fraction := strconv.FormatInt(minor%divisor, 10)
	return strconv.FormatInt(minor/divisor, 10) + "." + strings.Repeat("0", precision-len(fraction)) + fraction, nil
}
