// Customer self-isolation depends on the active audience and actual owner, not
// staff organization scopes. These PostgreSQL counterexamples keep every other
// identity, ownership and staff-scope gate in place.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// nativeCustomerScopeAuthorization grants the exact action, so a staff rejection
// cannot be explained by a missing write permission in this scope regression.
type nativeCustomerScopeAuthorization struct{ trafficTestAuthorization }

func (nativeCustomerScopeAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{authz.PermissionKey(PermissionSnapshotCreate): true}, nil
}

func TestNativeCustomerUnsupportedScopeReadPostgreSQL(t *testing.T) {
	service, identity, _, _ := nativeReadFixture(t)
	ctx := context.Background()
	nativeReadSeedOperation(t, service.database, "self-isolated-read", "native-a", "customer", "ready")
	nativeLifecycleIntent(t, service.database, "self-isolated-object", "native-a", "free", "succeeded")
	if err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteCreatedAccessory(ctx, service.database, servercommercialcap.Completion{
			OperationNo: "self-isolated-object", EntitlementNo: "native-a", ObjectNo: "self-object",
			Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now().UTC()})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET customer_no='other' WHERE entitlement_no='native-b'`); err != nil {
		t.Fatal(err)
	}
	ops := servercommercialcap.OperationListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 10}
	objects := servercommercialcap.AccessoryBillingListRequest{EntitlementNo: "native-a", CustomerNo: "customer", Limit: 10}
	for _, unsupported := range []bool{false, true} {
		service.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 0, DataScopeUnsupported: unsupported}}
		page, err := service.ListOperations(ctx, ops)
		if err != nil || page == nil || len(page.Items) != 2 {
			t.Fatalf("customer self history depends on staff scope unsupported=%v: %+v %v", unsupported, page, err)
		}
		billing, err := service.ListAccessoryBilling(ctx, objects)
		if err != nil || billing == nil || len(billing.Items) != 1 || billing.Items[0].ObjectNo != "self-object" {
			t.Fatalf("customer self billing depends on staff scope unsupported=%v: %+v %v", unsupported, billing, err)
		}
		foreignOps := ops
		foreignOps.EntitlementNo = "native-b"
		if _, err := service.ListOperations(ctx, foreignOps); err == nil {
			t.Fatal("customer read another owner's parent")
		}
		foreignObjects := objects
		foreignObjects.CustomerNo, foreignObjects.EntitlementNo = "other", "native-b"
		if _, err := service.ListAccessoryBilling(ctx, foreignObjects); err == nil {
			t.Fatal("customer supplied a foreign customer to read billing")
		}
	}
	identity.audience.Code, identity.audience.CustomerNo = principal.AudienceStaff, ""
	for _, current := range []bizctxcap.CurrentContext{
		{UserID: 101, DataScope: 0}, {UserID: 101, DataScope: 1, DataScopeUnsupported: true},
	} {
		service.pricing.bizCtx = trafficTestStaff{current}
		if _, err := service.ListOperations(ctx, ops); err == nil {
			t.Fatal("staff unsupported scope bypassed history permission")
		}
		if _, err := service.ListAccessoryBilling(ctx, objects); err == nil {
			t.Fatal("staff unsupported scope bypassed billing permission")
		}
	}
}

func TestNativeCustomerUnsupportedScopeCheckoutPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	f.native.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 0, DataScopeUnsupported: true}}
	quote, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionSnapshotCreate,
		Parameters: json.RawMessage(`{"name":"self-scope-object"}`), IdempotencyKey: "customer-self-scope-quote"})
	if err != nil || quote == nil {
		t.Fatalf("customer quote was blocked by staff scope: %+v %v", quote, err)
	}
	purchase, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: quote.QuoteNo, IdempotencyKey: "customer-self-scope-checkout"})
	if err != nil || purchase == nil || purchase.Status != "ready" {
		t.Fatalf("customer checkout was blocked by staff scope: %+v %v", purchase, err)
	}
	foreign := f.target
	foreign.CustomerNo = "CUS_other"
	if _, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: foreign, Action: serveroperationresourcecap.ActionSnapshotCreate,
		Parameters: json.RawMessage(`{"name":"foreign"}`), IdempotencyKey: "customer-foreign-scope-quote"}); err == nil {
		t.Fatal("unsupported scope allowed foreign customer quote")
	}
	f.native.pricing.principal = trafficTestPrincipal{}
	f.native.pricing.authorization = nativeCustomerScopeAuthorization{}
	f.native.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 1}}
	if err := f.native.authorizeNativeTarget(f.ctx, f.target.CustomerNo, serveroperationresourcecap.ActionSnapshotCreate); err != nil {
		t.Fatalf("precise staff action permission fixture is not valid: %v", err)
	}
	f.native.pricing.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 0, DataScopeUnsupported: true}}
	if err := f.native.authorizeNativeTarget(f.ctx, f.target.CustomerNo, serveroperationresourcecap.ActionSnapshotCreate); err == nil {
		t.Fatal("unsupported staff scope passed the actual action authorizer")
	}
	// A write permission is irrelevant when a staff scope is unsupported.
	if _, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionSnapshotCreate,
		Parameters: json.RawMessage(`{"name":"staff"}`), IdempotencyKey: "staff-unsupported-scope-quote"}); err == nil {
		t.Fatal("unsupported staff scope could quote")
	}
	if _, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: quote.QuoteNo, IdempotencyKey: "staff-unsupported-scope-checkout"}); err == nil {
		t.Fatal("unsupported staff scope could checkout")
	}
}
