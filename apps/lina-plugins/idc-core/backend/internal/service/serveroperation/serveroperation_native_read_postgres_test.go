// These read projections use actual 058/060/151 PostgreSQL constraints with
// bounded unrelated aggregate tables. Authentication is an explicit local
// boundary; providers, quotation services and financial writers are absent.

package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// nativeReadPrincipal binds tests to one explicitly selected audience.
type nativeReadPrincipal struct{ audience principal.Audience }

func (p *nativeReadPrincipal) Current(context.Context) (*principal.Audience, error) {
	copy := p.audience
	return &copy, nil
}

// nativeReadCustomer proves the independent current-customer owner check.
type nativeReadCustomer struct{ customer string }

func (c *nativeReadCustomer) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	return &identitycap.CustomerSubject{CustomerNo: c.customer, ActorUserID: 101}, nil
}

// nativeReadAuthorization grants only the legacy management read permission.
type nativeReadAuthorization struct {
	trafficTestAuthorization
	view bool
}

func (a *nativeReadAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{authz.PermissionKey(PermissionServerAccessoryView): a.view}, nil
}

// nativeReadFixture supplies no provider, pricing builder or invoice writer.
func nativeReadFixture(t *testing.T) (*nativeCommercialService, *nativeReadPrincipal, *nativeReadCustomer, *nativeReadAuthorization) {
	t.Helper()
	database := nativeLifecycleDB(t)
	if _, err := database.Exec(context.Background(), nativeCommercialReadBoundarySQL); err != nil {
		t.Fatal(err)
	}
	var (
		identity = &nativeReadPrincipal{principal.Audience{UserID: 101, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer"}}
		customer = &nativeReadCustomer{"customer"}
		grant    = &nativeReadAuthorization{view: true}
	)
	return &nativeCommercialService{database: database, pricing: &accessoryPricingImpl{database: database,
		bizCtx: trafficTestStaff{bizctxcap.CurrentContext{UserID: 101}}, principal: identity, customers: customer, authorization: grant}}, identity, customer, grant
}

// nativeReadSeedOperation inserts an immutable valid addon quote and a bounded
// business-operation root with an explicit real customer/subject identity.
func nativeReadSeedOperation(t *testing.T, database gdb.DB, number, parent, customer, status string) {
	t.Helper()
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_server_accessory_quote
(quote_no,customer_no,service_no,subject_type_code,subject_no,expected_subject_version,native_target_snapshot,
action_code,accessory_kind,currency_code,requested_quantity,existing_object_count,reserved_create_count,free_count_per_server,charged_quantity,unit_amount_minor,total_amount_minor,period_unit_code,period_count,policy_no,policy_version,idempotency_digest,request_digest,content_digest,expires_at,baseline_snapshot_no,baseline_observed_at)
SELECT ?,?,NULL,'entitlement',?,1,'{"localResourceNo":"local"}'::JSONB,
action_code,accessory_kind,currency_code,1,0,0,0,1,100,100,
period_unit_code,period_count,policy_no,policy_version,md5(?)||md5(?),request_digest,content_digest,expires_at,baseline_snapshot_no,baseline_observed_at
FROM plugin_idc_core_server_accessory_quote WHERE quote_no='old-quote'`, "quote-"+number, customer, parent, number, number); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_business_operation(operation_no,subject_type_code,subject_no,customer_no,currency_code,operation_type_code,status_code,quote_no)
VALUES(?,'entitlement',?,?,'CNY','addon',?,?)`, number, parent, customer, status, "quote-"+number); err != nil {
		t.Fatal(err)
	}
}

func TestNativeCommercialHistoryReadPostgreSQL(t *testing.T) {
	service, _, _, _ := nativeReadFixture(t)
	ctx := context.Background()
	for i := range 5 {
		nativeReadSeedOperation(t, service.database, "read-"+strconv.Itoa(i), "native-a", "customer", "manual_review")
	}
	nativeReadSeedOperation(t, service.database, "foreign-parent", "native-b", "customer", "ready")
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET last_error_code='provider.secret_token_123' WHERE operation_no='read-4';
UPDATE plugin_idc_core_business_operation SET last_error_code='server_commercial.ready_blocked.version_changed' WHERE operation_no='read-3';
UPDATE plugin_idc_core_service_entitlement SET status_code='expired',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 day' WHERE entitlement_no='native-a'`); err != nil {
		t.Fatal(err)
	}
	request := servercommercialcap.OperationListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 2}
	seen := make(map[string]bool)
	var firstCursor string
	for {
		page, err := service.ListOperations(ctx, request)
		if err != nil || page == nil || len(page.Items) > 2 {
			t.Fatalf("bounded history page: %+v %v", page, err)
		}
		for _, item := range page.Items {
			if seen[item.OperationNo] || item.OperationNo == "foreign-parent" || item.Status != "manual_review" || item.TotalAmountMinor != "100" || item.Action != serveroperationresourcecap.ActionSnapshotCreate || item.QuoteNo != "quote-"+item.OperationNo {
				t.Fatalf("history identity/state/precision changed: %+v", item)
			}
			if item.OperationNo == "read-4" && item.LastErrorCode != "" {
				t.Fatal("regex-valid provider secret escaped public projection")
			}
			if item.OperationNo == "read-3" && item.LastErrorCode != string(nativeReadyVersion) {
				t.Fatal("known local reason disappeared")
			}
			encoded, encodeErr := json.Marshal(item)
			if encodeErr != nil || !strings.Contains(string(encoded), `"TotalAmountMinor":"100"`) {
				t.Fatalf("minor units became a JSON number: %s %v", encoded, encodeErr)
			}
			seen[item.OperationNo] = true
		}
		if firstCursor == "" {
			firstCursor = page.NextCursor
		}
		if page.NextCursor == "" {
			break
		}
		request.Cursor = page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("keyset pagination lost records: %v", seen)
	}
	request.Cursor, request.OperationNo = "", "read-4"
	page, err := service.ListOperations(ctx, request)
	if err != nil || len(page.Items) != 1 || page.Items[0].OperationNo != "read-4" || page.NextCursor != "" {
		t.Fatalf("exact operation read failed: %+v %v", page, err)
	}
	for _, change := range []func(*servercommercialcap.OperationListRequest){
		func(in *servercommercialcap.OperationListRequest) { in.EntitlementNo = "native-b" },
		func(in *servercommercialcap.OperationListRequest) { in.OperationNo = "read-1" },
	} {
		bad := servercommercialcap.OperationListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 2, Cursor: firstCursor}
		change(&bad)
		if _, err := service.ListOperations(ctx, bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor accepted under changed scope: %v", err)
		}
	}
	request.OperationNo = "foreign-parent"
	page, err = service.ListOperations(ctx, request)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("foreign operation presence leaked: %+v %v", page, err)
	}
	request.EntitlementNo, request.OperationNo = "missing", ""
	if _, err := service.ListOperations(ctx, request); err == nil {
		t.Fatal("missing parent accepted")
	}
	// Actual JSON cap encoding retains minor units as quoted decimal text.
	encoded, err := json.Marshal(page)
	if err != nil || strings.Contains(string(encoded), "provider.secret") {
		t.Fatalf("private error escaped DTO: %s %v", encoded, err)
	}
}

func TestNativeCommercialBillingReadPostgreSQL(t *testing.T) {
	service, _, _, _ := nativeReadFixture(t)
	ctx := context.Background()
	for _, mode := range []string{"free", "paid"} {
		nativeLifecycleIntent(t, service.database, "billing-"+mode, "native-a", mode, "succeeded")
		if err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			return nativeCompleteCreatedAccessory(ctx, service.database, servercommercialcap.Completion{OperationNo: "billing-" + mode,
				EntitlementNo: "native-a", ObjectNo: "object-" + mode, Status: servercommercialcap.CompletionSucceeded,
				EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()})
		}); err != nil {
			t.Fatal(err)
		}
	}
	nativeLifecycleIntent(t, service.database, "unbound", "native-a", "paid", "manual_review")
	if _, err := service.database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice(invoice_no,customer_no,currency_code,status_code,total_amount_minor,paid_amount_minor)
VALUES('renewal','customer','CNY','pending_payment',9007199254740993,10);
UPDATE plugin_idc_core_server_accessory_entitlement SET pending_invoice_no='renewal',pending_policy_no='policy',pending_policy_version=1,
pending_period_unit_code='month',pending_period_count=1,pending_total_amount_minor=500 WHERE entitlement_no='accessory-billing-paid'`); err != nil {
		t.Fatal(err)
	}
	request := servercommercialcap.AccessoryBillingListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 1}
	first, err := service.ListAccessoryBilling(ctx, request)
	if err != nil || len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("bound object page: %+v %v", first, err)
	}
	paid := first.Items[0]
	if paid.AccessoryEntitlementNo != "accessory-billing-paid" || paid.ObjectNo != "object-paid" || paid.BillingMode != "paid" || paid.PendingAmountMinor == nil || *paid.PendingAmountMinor != "9007199254740993" || paid.PendingInvoiceNo != "renewal" || paid.CurrentPeriodStartAt == nil || paid.CurrentPeriodEndAt == nil {
		t.Fatalf("frozen invoice total/paid period lost: %+v", paid)
	}
	request.Cursor = first.NextCursor
	second, err := service.ListAccessoryBilling(ctx, request)
	if err != nil || len(second.Items) != 1 || second.NextCursor != "" {
		t.Fatalf("bound second page: %+v %v", second, err)
	}
	free := second.Items[0]
	if free.ObjectNo != "object-free" || free.BillingMode != "free" || free.PendingAmountMinor != nil || free.PendingInvoiceNo != "" || free.CurrentPeriodStartAt != nil || free.CurrentPeriodEndAt != nil {
		t.Fatalf("free object acquired invented billing: %+v", free)
	}
	for _, change := range []func(*servercommercialcap.AccessoryBillingListRequest){
		func(in *servercommercialcap.AccessoryBillingListRequest) { in.EntitlementNo = "native-b" },
		func(in *servercommercialcap.AccessoryBillingListRequest) {
			in.Kind = serveroperationresourcecap.AccessoryKindBackup
		},
		func(in *servercommercialcap.AccessoryBillingListRequest) { in.ObjectNo = "object-paid" },
	} {
		bad := request
		change(&bad)
		if _, err := service.ListAccessoryBilling(ctx, bad); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("object cursor accepted changed filters: %v", err)
		}
	}
	request.Cursor, request.ObjectNo, request.Kind = "", "object-paid", serveroperationresourcecap.AccessoryKindSnapshot
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_entitlement SET status_code='deleted',deleted_at=CURRENT_TIMESTAMP,auto_renew_enabled=FALSE WHERE entitlement_no='accessory-billing-paid'`); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListAccessoryBilling(ctx, request)
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != "deleted" || page.Items[0].AutoRenewEnabled {
		t.Fatalf("deleted binding was inferred active/absent: %+v %v", page, err)
	}
}

func TestNativeCommercialReadAuthorizationPostgreSQL(t *testing.T) {
	service, identity, customer, grant := nativeReadFixture(t)
	ctx := context.Background()
	for i := range 3 {
		nativeReadSeedOperation(t, service.database, "auth-read-"+strconv.Itoa(i), "native-a", "customer", "ready")
	}
	ops := servercommercialcap.OperationListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 2}
	objects := servercommercialcap.AccessoryBillingListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 2}
	assertDenied := func() {
		t.Helper()
		if _, err := service.ListOperations(ctx, ops); err == nil {
			t.Fatal("unprivileged history reader accepted")
		}
		if _, err := service.ListAccessoryBilling(ctx, objects); err == nil {
			t.Fatal("unprivileged object billing reader accepted")
		}
	}
	identity.audience.CustomerNo = "other"
	assertDenied()
	identity.audience.CustomerNo, customer.customer = "customer", "other"
	assertDenied()
	customer.customer = "customer"
	identity.audience.Status = principal.AudienceStatusRevoked
	assertDenied()
	identity.audience.Status, identity.audience.Code, identity.audience.CustomerNo = principal.AudienceStatusActive, principal.AudienceStaff, ""
	for _, scope := range []int{0, 3, 4, 5} {
		service.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: scope}}
		assertDenied()
	}
	for _, current := range []bizctxcap.CurrentContext{
		{UserID: 101, DataScope: 1, ActingUserID: 2}, {UserID: 101, DataScope: 1, IsImpersonation: true},
		{UserID: 101, DataScope: 1, ActingAsTenant: true}, {UserID: 101, DataScope: 1, TenantID: 2},
		{UserID: 101, DataScope: 1, DataScopeUnsupported: true}, {UserID: 2, DataScope: 1},
	} {
		service.pricing.bizCtx = trafficTestStaff{current}
		assertDenied()
	}
	service.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 1}}
	grant.view = false
	assertDenied()
	grant.view = true
	first, err := service.ListOperations(ctx, ops)
	if err != nil || first.NextCursor == "" {
		t.Fatalf("could not create first authorized page: %+v %v", first, err)
	}
	grant.view = false
	secondPage := ops
	secondPage.Cursor = first.NextCursor
	if _, err := service.ListOperations(ctx, secondPage); err == nil {
		t.Fatal("revoked permission could continue history pagination")
	}
	grant.view = true
	for _, scope := range []int{1, 2} {
		service.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: scope}}
		// A database-enforced read-only transaction catches any accidental
		// quote refresh, task write, audit append or cursor metadata mutation.
		if err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			if _, err := service.database.Exec(ctx, `SET TRANSACTION READ ONLY`); err != nil {
				return err
			}
			if _, err := service.ListOperations(ctx, ops); err != nil {
				return err
			}
			_, err := service.ListAccessoryBilling(ctx, objects)
			return err
		}); err != nil {
			t.Fatalf("read-only staff scope %d failed: %v", scope, err)
		}
	}
	for _, limit := range []int{0, 101} {
		invalid := ops
		invalid.Limit = limit
		if _, err := service.ListOperations(ctx, invalid); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("unbounded request accepted: %d %v", limit, err)
		}
	}
	if _, err := service.ListOperations(nil, ops); err == nil {
		t.Fatal("nil context accepted")
	}
	emptyOps := ops
	emptyOps.EntitlementNo = "native-b"
	empty, err := service.ListOperations(ctx, emptyOps)
	if err != nil || empty == nil || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("owned empty parent did not return empty array: %+v %v", empty, err)
	}
	emptyObjects := objects
	emptyObjects.EntitlementNo = "native-b"
	emptyBilling, err := service.ListAccessoryBilling(ctx, emptyObjects)
	if err != nil || emptyBilling == nil || emptyBilling.Items == nil || len(emptyBilling.Items) != 0 {
		t.Fatalf("owned empty parent billing did not return empty array: %+v %v", emptyBilling, err)
	}
}

func TestNativeCommercialPricedHistoryPrecisionPostgreSQL(t *testing.T) {
	service, _, _, _ := nativeReadFixture(t)
	ctx := context.Background()
	for _, action := range []string{"resize", "ip_change"} {
		if _, err := service.database.Exec(ctx, `INSERT INTO plugin_idc_core_channel_service_quote
(quote_no,source_code,service_no,service_owner_customer_no,payer_customer_no,currency_code,amount_minor)
VALUES(?,'native','native-a','customer','customer','CNY',9007199254740993)`, "quote-"+action); err != nil {
			t.Fatal(err)
		}
		if _, err := service.database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation(operation_no,subject_type_code,subject_no,customer_no,currency_code,operation_type_code,status_code,quote_no)
VALUES(?,'entitlement','native-a','customer','CNY',?,'manual_review',?)`, "read-"+action, action, "quote-"+action); err != nil {
			t.Fatal(err)
		}
	}
	page, err := service.ListOperations(ctx, servercommercialcap.OperationListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 10})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("priced history read failed: %+v %v", page, err)
	}
	for _, item := range page.Items {
		encoded, encodeErr := json.Marshal(item)
		if item.TotalAmountMinor != "9007199254740993" || encodeErr != nil || !strings.Contains(string(encoded), `"TotalAmountMinor":"9007199254740993"`) ||
			(item.Action != serveroperationresourcecap.ActionResize && item.Action != serveroperationresourcecap.ActionChangeIP) {
			t.Fatalf("priced operation amount/action changed: %+v %s %v", item, encoded, encodeErr)
		}
	}
}

func TestNativeCommercialReadTransferPrivacyPostgreSQL(t *testing.T) {
	service, identity, customer, _ := nativeReadFixture(t)
	ctx := context.Background()
	nativeReadSeedOperation(t, service.database, "old-owner", "native-a", "customer", "manual_review")
	nativeLifecycleIntent(t, service.database, "transferred", "native-a", "paid", "succeeded")
	if err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteCreatedAccessory(ctx, service.database, servercommercialcap.Completion{OperationNo: "transferred", EntitlementNo: "native-a",
			ObjectNo: "transferred-object", Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_order(order_no,customer_no,currency_code) VALUES('old-order','customer','CNY');
INSERT INTO plugin_idc_core_billing_invoice(invoice_no,order_no,customer_no,currency_code,status_code,total_amount_minor)
VALUES('old-invoice-private','old-order','customer','CNY','pending_payment',9007199254740993);
UPDATE plugin_idc_core_service_entitlement SET customer_no='other',status_code='suspended' WHERE entitlement_no='native-a';
UPDATE plugin_idc_core_server_accessory_entitlement SET customer_no='other',pending_invoice_no='old-invoice-private',pending_policy_no='policy',pending_policy_version=1,
pending_period_unit_code='month',pending_period_count=1,pending_total_amount_minor=9007199254740993 WHERE entitlement_no='accessory-transferred'`); err != nil {
		t.Fatal(err)
	}
	oldRequest := servercommercialcap.OperationListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 100}
	if _, err := service.ListOperations(ctx, oldRequest); err == nil {
		t.Fatal("former owner could still read transferred entitlement history")
	}
	identity.audience.CustomerNo, customer.customer = "other", "other"
	nativeReadSeedOperation(t, service.database, "new-owner", "native-a", "other", "manual_review")
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET order_no='old-order',invoice_no='old-invoice-private',last_error_code='server_commercial.ready_admission.object_identity_unknown' WHERE operation_no='new-owner'`); err != nil {
		t.Fatal(err)
	}
	oldRequest.CustomerNo = "other"
	page, err := service.ListOperations(ctx, oldRequest)
	if err != nil || len(page.Items) != 1 || page.Items[0].OperationNo != "new-owner" || page.Items[0].Status != "manual_review" || page.Items[0].LastErrorCode != "server_commercial.ready_admission.object_identity_unknown" || page.Items[0].OrderNo != "" || page.Items[0].InvoiceNo != "" {
		t.Fatalf("transfer leaked old financial history/reinterpreted unknown: %+v %v", page, err)
	}
	objects, err := service.ListAccessoryBilling(ctx, servercommercialcap.AccessoryBillingListRequest{EntitlementNo: "native-a", CustomerNo: "other", Limit: 100})
	if err != nil || len(objects.Items) != 1 || objects.Items[0].PendingInvoiceNo != "" || objects.Items[0].PendingAmountMinor != nil || objects.Items[0].PendingInvoiceStatus != "" {
		t.Fatalf("new owner received old customer's pending invoice: %+v %v", objects, err)
	}
	// Currency mismatch is equally invisible, even for the same current owner.
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_billing_invoice SET customer_no='other',currency_code='USD' WHERE invoice_no='old-invoice-private'`); err != nil {
		t.Fatal(err)
	}
	objects, err = service.ListAccessoryBilling(ctx, servercommercialcap.AccessoryBillingListRequest{EntitlementNo: "native-a", CustomerNo: "other", Limit: 100})
	if err != nil || len(objects.Items) != 1 || objects.Items[0].PendingInvoiceNo != "" || objects.Items[0].PendingAmountMinor != nil {
		t.Fatalf("mismatched-currency pending amount escaped: %+v %v", objects, err)
	}
}

func TestNativeCommercialReadRealPricedCheckoutPostgreSQL(t *testing.T) {
	f, _ := newNativePricedFixture(t, 1000)
	quote, purchase := nativePricedIPCheckout(t, f)
	page, err := f.native.ListOperations(f.ctx, servercommercialcap.OperationListRequest{
		EntitlementNo: f.target.EntitlementNo, CustomerNo: f.target.CustomerNo, OperationNo: purchase.OperationNo, Limit: 10})
	if err != nil || page == nil || len(page.Items) != 1 {
		t.Fatalf("real 152 quote/checkout not readable: %+v %v", page, err)
	}
	item := page.Items[0]
	if item.OperationNo != purchase.OperationNo || item.QuoteNo != quote.QuoteNo || item.Action != serveroperationresourcecap.ActionChangeIP ||
		item.TotalAmountMinor != strconv.FormatInt(quote.TotalAmountMinor, 10) || item.OrderNo != purchase.OrderNo || item.InvoiceNo != purchase.InvoiceNo || item.Status != purchase.Status {
		t.Fatalf("real frozen checkout projection changed: %+v / %+v", item, purchase)
	}
	var state struct{ StatusCode string }
	if err := f.database.GetScan(f.ctx, &state, `SELECT status_code FROM plugin_idc_core_business_operation WHERE operation_no=?`, purchase.OperationNo); err != nil || state.StatusCode != item.Status {
		t.Fatalf("history read mutated actual operation: %+v %v", state, err)
	}
	// The real completion owner maps an uncertain resource receipt to
	// manual_review. History must preserve that state, not invent a different
	// business-operation enum or a successful completion timestamp.
	nativeRecoverySettle(t, f, purchase, false)
	if err := f.native.WithReadyOperation(f.ctx, purchase.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) {
		return "history-unknown-task", nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.native.Complete(f.ctx, servercommercialcap.Completion{OperationNo: purchase.OperationNo,
		EntitlementNo: f.target.EntitlementNo, TaskNo: "history-unknown-task", Status: servercommercialcap.CompletionUnknown,
		EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	page, err = f.native.ListOperations(f.ctx, servercommercialcap.OperationListRequest{
		EntitlementNo: f.target.EntitlementNo, CustomerNo: f.target.CustomerNo, OperationNo: purchase.OperationNo, Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].Status != "manual_review" || page.Items[0].CompletedAt != nil {
		t.Fatalf("actual unknown receipt misrepresented: %+v %v", page, err)
	}
}

// nativeCommercialReadBoundarySQL bounds only roots not exercised by these reads.
const nativeCommercialReadBoundarySQL = `
ALTER TABLE plugin_idc_core_business_operation ADD COLUMN quote_no VARCHAR(40),ADD COLUMN order_no VARCHAR(40),ADD COLUMN invoice_no VARCHAR(40),
 ADD COLUMN last_error_code TEXT DEFAULT '',ADD COLUMN created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,ADD COLUMN completed_at TIMESTAMPTZ;
CREATE TABLE plugin_idc_core_commerce_order(tenant_id INT DEFAULT 0,order_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40),currency_code CHAR(3));
CREATE TABLE plugin_idc_core_channel_service_quote(tenant_id INT DEFAULT 0,quote_no VARCHAR(40) PRIMARY KEY,source_code TEXT,service_no VARCHAR(40),service_owner_customer_no VARCHAR(40),payer_customer_no VARCHAR(40),currency_code CHAR(3),amount_minor BIGINT);
`
