// These real-PG checks isolate the source projection as a read-only boundary,
// then change owner facts or staff permissions while candidate I/O is running.
package serveroperation

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"testing"
)

type nativeContextCatalogFixture struct {
	catalog.CatalogReader
	read func(context.Context, catalog.OwnedPurchaseConfigurationInput) (*catalog.OwnedPurchaseConfigurationContext, error)
}

func (f nativeContextCatalogFixture) ReadOwnedPurchaseConfiguration(ctx context.Context, in catalog.OwnedPurchaseConfigurationInput) (*catalog.OwnedPurchaseConfigurationContext, error) {
	return f.read(ctx, in)
}

type nativeContextAuthorization struct {
	trafficTestAuthorization
	edit bool
}

func (a *nativeContextAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{authz.PermissionKey(PermissionServerEdit): a.edit}, nil
}

func TestNativeConfigurationContextRechecksIdentityAfterIOPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	calls := 0
	f.native.prices.catalog = nativeContextCatalogFixture{read: func(_ context.Context, in catalog.OwnedPurchaseConfigurationInput) (*catalog.OwnedPurchaseConfigurationContext, error) {
		calls++
		if in.EntitlementNo != f.target.EntitlementNo || in.CustomerNo != f.target.CustomerNo {
			t.Fatal("catalog received a forged owner")
		}
		return &catalog.OwnedPurchaseConfigurationContext{PublicPurchaseConfigurationEvaluation: catalog.PublicPurchaseConfigurationEvaluation{PublicPurchaseConfigurationView: catalog.PublicPurchaseConfigurationView{ConfigurationRevision: "opaque-revision", Fields: []catalog.PublicPurchaseConfigurationField{}, Values: []catalog.PublicPurchaseConfigurationValue{}}, Valid: false, Issues: []catalog.PublicPurchaseConfigurationIssue{{FieldToken: "opaque-field", IssueCode: "required"}}}}, nil
	}}
	input := servercommercialcap.ConfigurationContextRequest{Target: f.target}
	result, err := f.native.ReadConfigurationContext(f.ctx, input)
	if err != nil || result == nil || result.Ready || result.BillingTermToken != "" || len(result.Issues) != 1 {
		t.Fatalf("invalid candidate context=%+v / %+v", result, err)
	}
	if _, err = f.native.ReadConfigurationContext(nil, input); err == nil {
		t.Fatal("nil context accepted")
	}
	before := calls
	if err = f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := f.native.ReadConfigurationContext(ctx, input); err == nil {
			t.Fatal("outer transaction accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if calls != before {
		t.Fatal("transaction rejection performed candidate IO")
	}
	foreign := input
	foreign.Target.CustomerNo = "CUS_foreign"
	if _, err = f.native.ReadConfigurationContext(f.ctx, foreign); err == nil {
		t.Fatal("foreign customer accepted")
	}
	f.native.prices.catalog = nativeContextCatalogFixture{read: func(ctx context.Context, _ catalog.OwnedPurchaseConfigurationInput) (*catalog.OwnedPurchaseConfigurationContext, error) {
		_, err := f.database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended',version=version+1 WHERE entitlement_no=?`, f.target.EntitlementNo)
		if err != nil {
			return nil, err
		}
		return &catalog.OwnedPurchaseConfigurationContext{}, nil
	}}
	if _, err = f.native.ReadConfigurationContext(f.ctx, input); err == nil {
		t.Fatal("parent changed during candidate IO but context returned")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_order`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 0)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_native_server_target_quote`, 0)
}

func TestNativeConfigurationContextRechecksStaffGrantPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixture(t)
	f.native.pricing.principal = trafficTestPrincipal{}
	authorization := &nativeContextAuthorization{edit: true}
	f.native.pricing.authorization = authorization
	f.native.prices.catalog = nativeContextCatalogFixture{read: func(context.Context, catalog.OwnedPurchaseConfigurationInput) (*catalog.OwnedPurchaseConfigurationContext, error) {
		authorization.edit = false
		return &catalog.OwnedPurchaseConfigurationContext{}, nil
	}}
	if _, err := f.native.ReadConfigurationContext(f.ctx, servercommercialcap.ConfigurationContextRequest{Target: f.target}); err == nil {
		t.Fatal("revoked staff retained context")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 0)
}
