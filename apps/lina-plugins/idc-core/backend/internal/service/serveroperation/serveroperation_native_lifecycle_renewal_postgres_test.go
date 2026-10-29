// Renewal tests exercise the native scanner and retained billing state machine
// with real PostgreSQL and an explicit non-spending invoice payment substitute.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// nativeRenewalPayment refuses to spend real funds. Tests may later mark the
// fixture invoice settled to represent the authoritative billing boundary.
type nativeRenewalPayment struct {
	billing.InvoiceService
	calls     int
	beforePay func(context.Context) error
}

func (payment *nativeRenewalPayment) PayFromBalance(ctx context.Context, _ billing.InvoiceBalancePaymentInput) (*billing.InvoiceBalancePayment, error) {
	if payment.beforePay != nil {
		if err := payment.beforePay(ctx); err != nil {
			return nil, err
		}
	}
	payment.calls++
	return nil, bizerr.NewCode(billing.CodeInsufficientBalance)
}

// nativeRenewalNotifications captures only local, transactional notification intents.
type nativeRenewalNotifications struct {
	requests []notificationcap.EventRequest
	database gdb.DB
	fail     bool
}

func (sink *nativeRenewalNotifications) Enqueue(ctx context.Context, input notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	if sink.fail && input.EventCode == accessoryRenewalReminderEvent {
		return notificationcap.EventReceipt{}, ErrServiceUnavailable
	}
	if gdb.TXFromCtx(ctx, sink.database.GetGroup()) == nil {
		return notificationcap.EventReceipt{}, ErrOperationConflict
	}
	variables, err := json.Marshal(input.Variables)
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	if _, err = sink.database.Exec(ctx, `INSERT INTO native_notification_intent(event_key,variables) VALUES(?,?::JSONB) ON CONFLICT(event_key) DO NOTHING`, input.IdempotencyKey, string(variables)); err != nil {
		return notificationcap.EventReceipt{}, err
	}
	sink.requests = append(sink.requests, input)
	return notificationcap.EventReceipt{}, nil
}

func TestNativeLifecycleRenewalPaymentAndDeletionPostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "paid", "native-a", "paid", "succeeded")
	input := servercommercialcap.Completion{OperationNo: "paid", EntitlementNo: "native-a", ObjectNo: "paid-object", EvidenceDigest: strings.Repeat("b", 64), Status: servercommercialcap.CompletionSucceeded, ObservedAt: time.Now().UTC().AddDate(0, -2, 0)}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, input) }); err != nil {
		t.Fatal(err)
	}
	nativeLifecycleIntent(t, database, "grandfather", "native-a", "free", "succeeded")
	grandfather := input
	grandfather.OperationNo, grandfather.ObjectNo = "grandfather", "grandfather-object"
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteCreatedAccessory(ctx, database, grandfather)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact(tenant_id,customer_no,contact_no,status_code) VALUES(0,'customer','billing-contact','active');
INSERT INTO plugin_idc_core_customer_contact_role VALUES(0,'customer','billing-contact','billing','active')`); err != nil {
		t.Fatal(err)
	}
	payment := &nativeRenewalPayment{beforePay: func(ctx context.Context) error {
		committed, err := database.GetValue(ctx, `SELECT count(*) FROM native_notification_intent WHERE event_key LIKE 'accessory-renewal-before-payment:%' AND variables->>'amount'='1.00' AND variables->>'currency'='CNY'`)
		if err != nil || committed.Int() != 1 {
			t.Fatal("automatic debit preceded committed frozen amount reminder", err)
		}
		return nil
	}}
	sink := &nativeRenewalNotifications{database: database}
	worker := &workerImpl{database: database, invoices: payment, notifications: sink}
	renewed, pending, err := worker.reconcileAccessoryRenewals(ctx, 20)
	if err != nil || renewed != 0 || pending != 1 || payment.calls != 1 || len(sink.requests) != 2 {
		t.Fatalf("native renewal was skipped or spent real funds: %d/%d calls=%d notify=%d err=%+v", renewed, pending, payment.calls, len(sink.requests), err)
	}
	if sink.requests[1].EventCode != accessoryRenewalFailureEvent || len(sink.requests[1].Variables) != 1 || sink.requests[1].Variables["invoice_no"] == "" {
		t.Fatalf("wrong renewal failure event: %+v", sink.requests[1])
	}
	var frozen *struct {
		InvoiceNo, StatusCode string
		Amount                int64
	}
	if err = database.GetScan(ctx, &frozen, `SELECT accessory.pending_invoice_no AS invoice_no,accessory.status_code,
invoice.total_amount_minor AS amount FROM plugin_idc_core_server_accessory_entitlement accessory
JOIN plugin_idc_core_billing_invoice invoice ON invoice.invoice_no=accessory.pending_invoice_no
WHERE accessory.entitlement_no='accessory-paid'`); err != nil || frozen == nil || frozen.Amount != 100 || frozen.StatusCode != "payment_pending" {
		t.Fatalf("unpaid object lost frozen invoice: %+v %v", frozen, err)
	}
	work, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_renewal_work_item
WHERE subject_type_code='entitlement' AND subject_no='native-a' AND service_no IS NULL
AND required_permission=? AND status_code='open'`, PermissionAccessoryRenewalHandle)
	if err != nil || work.Int() != 1 {
		t.Fatal("native renewal work item lost subject or permission", err)
	}
	object := servercommercialcap.ObjectRequest{EntitlementNo: "native-a", CustomerNo: "customer", Kind: "snapshot", ObjectNo: "paid-object"}
	if err = nativeCheckRestore(ctx, database, object); err == nil {
		t.Fatal("unpaid object remained usable")
	}
	// Published changes cannot rewrite the already-issued 100-minor invoice.
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_price_policy_version
(policy_no,policy_version,accessory_kind,currency_code,free_count_per_server,unit_amount_minor,period_unit_code,period_count,content_digest,published_by)
VALUES('policy',2,'snapshot','CNY',0,250,'month',1,repeat('c',64),1);
UPDATE plugin_idc_core_server_accessory_price_policy SET current_version=2 WHERE policy_no='policy'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	free, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement
WHERE entitlement_no='accessory-grandfather' AND billing_mode_code='free' AND status_code='active'
AND current_period_start_at IS NULL AND current_period_end_at IS NULL AND pending_invoice_no IS NULL`)
	if err != nil || free.Int() != 1 {
		t.Fatal("free quota decrease retroactively charged an existing free object", err)
	}
	settledAt := databaseTime(time.Now().UTC().Add(-24 * time.Hour))
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice SET status_code='paid',settled_at=? WHERE invoice_no=?`, settledAt, frozen.InvoiceNo); err != nil {
		t.Fatal(err)
	}
	renewed, pending, err = worker.reconcileAccessoryRenewals(ctx, 20)
	if err != nil || renewed != 1 || pending != 0 || payment.calls != 1 {
		t.Fatalf("settled invoice did not reactivate original object: %d/%d %v", renewed, pending, err)
	}
	var period struct {
		CurrentPeriodStartAt, CurrentPeriodEndAt time.Time
	}
	if err = database.GetScan(ctx, &period, `SELECT current_period_start_at,current_period_end_at FROM plugin_idc_core_server_accessory_entitlement WHERE entitlement_no='accessory-paid'`); err != nil {
		t.Fatal(err)
	}
	wantEnd := settledAt.AddDate(0, 1, 0)
	if period.CurrentPeriodStartAt.Sub(settledAt).Abs() > time.Microsecond || period.CurrentPeriodEndAt.Sub(wantEnd).Abs() > time.Microsecond {
		t.Fatalf("delayed scanner moved the paid cycle: got %s..%s, paid %s", period.CurrentPeriodStartAt, period.CurrentPeriodEndAt, settledAt)
	}
	if err = nativeCheckRestore(ctx, database, object); err != nil {
		t.Fatal("paid object was not restored", err)
	}
	input.OperationNo, input.ObservedAt = "delete", time.Now().UTC()
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteDeletedAccessory(ctx, database, object, input)
	}); err != nil {
		t.Fatal(err)
	}
	renewed, pending, err = worker.reconcileAccessoryRenewals(ctx, 20)
	if err != nil || renewed != 0 || pending != 0 || payment.calls != 1 {
		t.Fatal("deleted object continued renewing", err)
	}
	amount, err := database.GetValue(ctx, `SELECT total_amount_minor FROM plugin_idc_core_billing_invoice WHERE invoice_no=?`, frozen.InvoiceNo)
	if err != nil || amount.Int64() != 100 {
		t.Fatal("new policy changed an issued or settled historical bill", err)
	}
}

func TestNativeLifecycleRenewalTaxSnapshotPostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_commerce_order_item SET tax_treatment_code='taxable',tax_rate_basis_points=1000;
INSERT INTO plugin_idc_core_service_price_snapshot(price_snapshot_no,subject_type_code,subject_no,tax_treatment_code,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor)
VALUES('effective','entitlement','native-a','exempt','cloud_service','line','half_up',1);
UPDATE plugin_idc_core_service_entitlement SET current_price_snapshot_no='effective' WHERE entitlement_no='native-a'`); err != nil {
		t.Fatal(err)
	}
	price, err := loadNativeAccessoryRenewalPrice(ctx, database, &dueAccessoryRenewal{SubjectTypeCode: "entitlement", SubjectNo: "native-a", CustomerNo: "customer", CurrencyCode: "CNY", AccessoryKind: "snapshot"})
	if err != nil || price == nil || price.TaxTreatmentCode != "exempt" || price.TaxRateBasisPoints != nil {
		t.Fatalf("effective exempt snapshot inherited historical tax: %+v %v", price, err)
	}
}

func TestNativeLifecycleReminderFailureStopsDebitPostgreSQL(t *testing.T) {
	for _, failure := range []string{"enqueue", "missing-contact"} {
		t.Run(failure, func(t *testing.T) {
			database := nativeLifecycleDB(t)
			ctx := context.Background()
			_, _, invoice := nativeDeletionBill(t, database)
			addContact := func() {
				if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact(tenant_id,customer_no,contact_no,status_code) VALUES(0,'customer','billing-contact','active');
INSERT INTO plugin_idc_core_customer_contact_role VALUES(0,'customer','billing-contact','billing','active')`); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "enqueue" {
				addContact()
			}
			payment := &nativeRenewalPayment{}
			sink := &nativeRenewalNotifications{database: database, fail: failure == "enqueue"}
			worker := &workerImpl{database: database, invoices: payment, notifications: sink}
			if _, _, err := worker.reconcileAccessoryRenewals(ctx, 20); err == nil || payment.calls != 0 {
				t.Fatalf("failed reminder did not stop debit: calls=%d err=%v", payment.calls, err)
			}
			kept, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE status_code='payment_pending' AND pending_invoice_no=? AND object_no='object'`, invoice)
			if err != nil || kept.Int() != 1 {
				t.Fatal("reminder failure discarded the existing bill or data", err)
			}
			if failure == "missing-contact" {
				addContact()
			}
			sink.fail = false
			for range 2 {
				if _, _, err = worker.reconcileAccessoryRenewals(ctx, 20); err != nil {
					t.Fatal(err)
				}
			}
			count, err := database.GetValue(ctx, `SELECT count(*) FROM native_notification_intent WHERE event_key LIKE 'accessory-renewal-before-payment:%'`)
			if err != nil || count.Int() != 1 || payment.calls != 1 {
				t.Fatalf("retry duplicated reminder/debit: events=%v calls=%d err=%v", count, payment.calls, err)
			}
		})
	}
}
