// This file exercises fixed traffic packages against the real migration schema.
// All writes stay in a nonce-scoped fixture; provider capability and staff identity
// are explicit boundaries, while quote, checkout, stock and catalog use real SQL.

package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/dialect"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// trafficTestStaff is the direct staff request boundary, never a forged customer.
type trafficTestStaff struct{ current bizctxcap.CurrentContext }

func (staff trafficTestStaff) Current(context.Context) bizctxcap.CurrentContext { return staff.current }

// trafficTestPrincipal supplies an active staff audience for settings tests.
type trafficTestPrincipal struct{}

func (trafficTestPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 101, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

// trafficTestAuthorization grants only the existing accessory settings keys.
type trafficTestAuthorization struct{ authz.Service }

func (trafficTestAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{authz.PermissionKey(PermissionAccessoryPriceRead): true, authz.PermissionKey(PermissionAccessoryPriceWrite): true}, nil
}

func (trafficTestAuthorization) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return false, nil
}

// trafficTestResource is a read-only upstream capability, not an execution stub.
type trafficTestResource struct{ expiresAt time.Time }

func (resource trafficTestResource) GetAccessories(_ context.Context, route serveroperationresourcecap.CapabilityRoute) (*serveroperationresourcecap.AccessoryProjection, error) {
	return &serveroperationresourcecap.AccessoryProjection{ServiceNo: route.ServiceNo, SnapshotNo: "CAP_traffic_test", ObservedAt: time.Now().UTC(), ExpiresAt: resource.expiresAt, Actions: []serveroperationresourcecap.Action{serveroperationresourcecap.ActionTrafficAdd}}, nil
}

func (trafficTestResource) ValidateParameters(_ context.Context, _ serveroperationresourcecap.CapabilityRoute, action serveroperationresourcecap.Action, raw json.RawMessage) error {
	var value struct {
		TrafficGB int `json:"trafficGb"`
	}
	if action != serveroperationresourcecap.ActionTrafficAdd || json.Unmarshal(raw, &value) != nil || value.TrafficGB < 1 {
		return errors.New("invalid frozen traffic parameters")
	}
	return nil
}

// trafficTestAudit is concurrency-safe and never emits external messages.
type trafficTestAudit struct{ idccap.BusinessAuditWriter }

func (trafficTestAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return nil
}

// newTrafficPackageFixture reuses full real financial/server migrations, then
// supplies only the authoritative upstream/identity boundaries needed by Preview.
func newTrafficPackageFixture(t *testing.T) accessoryCheckoutFixture {
	t.Helper()
	fixture := newAccessoryCheckoutFixture(t)
	prepareAccessoryOriginalServer(t, fixture)
	applyTrafficFixtureMigration(t, fixture, "087-generic-refund-entitlement.sql")
	applyTrafficFixtureMigration(t, fixture, "088-generic-entitlement-renewal.sql")
	reader, err := catalog.NewPublicCatalogReader(fixture.database, []byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatal(err)
	}
	fixture.service.catalog = reader
	fixture.service.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 1}}
	fixture.service.principal = trafficTestPrincipal{}
	fixture.service.authorization = trafficTestAuthorization{}
	fixture.service.audit = trafficTestAudit{}
	fixture.service.customers = accessoryChainCustomer{}
	resource := trafficTestResource{time.Now().UTC().Add(time.Hour)}
	fixture.service.directory, fixture.service.parameters = resource, resource
	prepareTrafficProductBinding(t, fixture, "prd_checkout_test", "ver_checkout_test", "idc-resource", true, true)
	if _, err = fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published',published_at=CURRENT_TIMESTAMP WHERE version_no='ver_checkout_test'`); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_service_instance SET current_price_snapshot_no=(SELECT price_snapshot_no FROM plugin_idc_core_service_price_snapshot WHERE service_no='SRV_checkout_original' LIMIT 1) WHERE service_no='SRV_checkout_original'`); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// applyTrafficFixtureMigration keeps newer entitlement owner columns and stock
// guards identical to production rather than inventing a reduced table schema.
func applyTrafficFixtureMigration(t *testing.T, fixture accessoryCheckoutFixture, name string) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve traffic migration")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name))
	if err != nil {
		t.Fatal(err)
	}
	for i, statement := range dialect.SplitSQLStatements(string(content)) {
		if _, err = fixture.database.Exec(fixture.ctx, statement); err != nil {
			t.Fatalf("traffic migration %s statement %d: %+v", name, i+1, err)
		}
	}
}

// prepareTrafficProductBinding creates exact frozen plugin evidence on a draft
// before making that version current. It never adds deprecated server rows.
func prepareTrafficProductBinding(t *testing.T, fixture accessoryCheckoutFixture, productNo, versionNo, owner string, ongoing, bound bool) {
	t.Helper()
	const interfaceNo = "pfi_11111111111111111111111111111111"
	if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_product_automation_interface_instance
(interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,created_by,changed_by)
VALUES (?,'Synthetic resource connector','fixture-server','product-fulfillment','v1.0.0','v1',ARRAY['provision','query'],repeat('a',64),repeat('b',64),101,101) ON CONFLICT (interface_instance_no) DO NOTHING`, interfaceNo); err != nil {
		t.Fatal(err)
	}
	if bound {
		if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
(version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by)
VALUES (?,?,'Synthetic resource connector','fixture-server','product-fulfillment','v1.0.0','v1',ARRAY['provision','query'],repeat('a',64),repeat('b',64),101)`, versionNo, interfaceNo); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
(version_no,strategy_code,granularity_code,provision_method,query_method,policy_digest,created_by,ongoing_management,entitlement_owner_id)
VALUES (?,'payment_confirmed_plugin','per_quantity_unit','provision','query',repeat('c',64),101,?,?)`, versionNo, ongoing, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product SET current_version_no=? WHERE product_no=?`, versionNo, productNo); err != nil {
		t.Fatal(err)
	}
}

// trafficInput creates one explicit fixed merchant package; nil stock is unlimited.
func trafficInput() TrafficPackageInput {
	return TrafficPackageInput{NameZhCN: "固定100G", NameEnUS: "Fixed 100G", CurrencyCode: "CNY", TrafficGB: 100, AmountMinor: 1234, Enabled: true, ProductNos: []string{"prd_checkout_test"}}
}

func TestTrafficPackageValidation(t *testing.T) {
	input := trafficInput()
	if err := validateTrafficPackageInput(input); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*TrafficPackageInput){
		"empty scope": func(v *TrafficPackageInput) { v.ProductNos = nil },
		"over scope": func(v *TrafficPackageInput) {
			for i := 0; i < 101; i++ {
				v.ProductNos = append(v.ProductNos, fmt.Sprintf("prd_%d", i))
			}
		},
		"duplicate scope": func(v *TrafficPackageInput) { v.ProductNos = []string{"prd_checkout_test", "prd_checkout_test"} },
		"zero capacity":   func(v *TrafficPackageInput) { v.TrafficGB = 0 },
		"negative price":  func(v *TrafficPackageInput) { v.AmountMinor = -1 },
		"negative stock":  func(v *TrafficPackageInput) { value := int64(-1); v.StockQuantity = &value },
		"currency":        func(v *TrafficPackageInput) { v.CurrencyCode = "cny" },
		"new version":     func(v *TrafficPackageInput) { v.ExpectedVersion = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			value := trafficInput()
			mutate(&value)
			if !bizerr.Is(validateTrafficPackageInput(value), CodeAccessoryPriceInvalid) {
				t.Fatal("invalid package accepted")
			}
		})
	}
	input.ProductNos = nil
	for i := 0; i < 100; i++ {
		input.ProductNos = append(input.ProductNos, fmt.Sprintf("prd_%d", i))
	}
	if err := validateTrafficPackageInput(input); err != nil {
		t.Fatalf("100 product boundary: %v", err)
	}
}

// TestTrafficPackageSettingsRejectUnsafeContext proves authorization precedes
// even database access; the nil database would panic if the guard were bypassed.
func TestTrafficPackageSettingsRejectUnsafeContext(t *testing.T) {
	for name, current := range map[string]bizctxcap.CurrentContext{
		"anonymous": {}, "cross tenant": {UserID: 101, TenantID: 2, DataScope: 1},
		"assistance":        {UserID: 101, DataScope: 1, ActingUserID: 102},
		"unsupported scope": {UserID: 101, DataScope: 3},
	} {
		t.Run(name, func(t *testing.T) {
			service := &accessoryPricingImpl{bizCtx: trafficTestStaff{current}}
			if _, err := service.ListTrafficPackages(context.Background(), TrafficPackageSearch{}); !bizerr.Is(err, CodeAccessoryPriceNotAccessible) {
				t.Fatalf("unsafe read accepted: %v", err)
			}
			if _, err := service.SaveTrafficPackage(context.Background(), trafficInput()); !bizerr.Is(err, CodeAccessoryPriceNotAccessible) {
				t.Fatalf("unsafe write accepted: %v", err)
			}
		})
	}
	service := &accessoryPricingImpl{}
	if _, err := service.Preview(nil, AccessoryQuoteInput{Action: serveroperationresourcecap.ActionTrafficAdd}); !bizerr.Is(err, CodeAccessoryPriceInvalid) {
		t.Fatalf("nil quote context accepted: %v", err)
	}
}

func TestTrafficPackageHistoryAndStockPostgreSQL(t *testing.T) {
	fixture := newTrafficPackageFixture(t)
	applyTrafficFixtureMigration(t, fixture, "109-server-accessory-settlement-index.sql")
	input := trafficInput()
	created, err := fixture.service.SaveTrafficPackage(fixture.ctx, input)
	if err != nil {
		t.Fatalf("create fixed package: %+v", err)
	}
	input.PackageNo, input.ExpectedVersion = created.PackageNo, created.Version
	limit := int64(3)
	input.StockQuantity = &limit
	input.TrafficGB, input.AmountMinor, input.NameEnUS = 200, 2345, "Updated 200G"
	updated, err := fixture.service.SaveTrafficPackage(fixture.ctx, input)
	if err != nil || updated.Version != 2 {
		t.Fatalf("new immutable version: %+v / %+v", updated, err)
	}
	if _, err = fixture.service.SaveTrafficPackage(fixture.ctx, input); !bizerr.Is(err, CodeAccessoryPriceConflict) {
		t.Fatalf("stale edit accepted: %+v", err)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_server_accessory_price_policy_version WHERE policy_no='`+created.PackageNo+`' AND policy_version=1 AND traffic_gb=100 AND unit_amount_minor=1234 AND name_en_us='Fixed 100G'`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_traffic_package_product WHERE policy_no='`+created.PackageNo+`'`, 2)
	for _, query := range []string{
		`UPDATE plugin_idc_core_server_accessory_price_policy_version SET traffic_gb=1 WHERE policy_no=?`,
		`DELETE FROM plugin_idc_core_traffic_package_product WHERE policy_no=?`,
		`UPDATE plugin_idc_core_traffic_package_product SET product_no=product_no WHERE policy_no=?`,
	} {
		if _, err = fixture.database.Exec(fixture.ctx, query, created.PackageNo); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("history rewrite not rejected: %+v", err)
		}
	}
	input.ExpectedVersion = updated.Version
	input.StockQuantity = nil
	if _, err = fixture.service.SaveTrafficPackage(fixture.ctx, input); err != nil {
		t.Fatalf("clear stock limit: %+v", err)
	}
	page, err := fixture.service.ListTrafficPackages(fixture.ctx, TrafficPackageSearch{Keyword: "Updated 200G"})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].StockQuantity != nil || page.Items[0].RemainingQuantity != nil {
		t.Fatalf("unlimited projection: %+v / %+v", page, err)
	}
	// Existing unpaid purchases remain occupied when the total is reduced to zero.
	checkout, err := fixture.service.CheckoutCurrentCustomer(fixture.ctx, AccessoryCheckoutInput{QuoteNo: fixture.row.QuoteNo, IdempotencyKey: "traffic-history-checkout"})
	if err != nil {
		t.Fatalf("reserve seed package: %+v", err)
	}
	input = trafficInput()
	input.PackageNo = "POL_checkout_test"
	input.ExpectedVersion = 1
	zero := int64(0)
	input.StockQuantity = &zero
	input.Enabled = false
	if _, err = fixture.service.SaveTrafficPackage(fixture.ctx, input); err != nil {
		t.Fatalf("lower sold stock: %+v", err)
	}
	page, err = fixture.service.ListTrafficPackages(fixture.ctx, TrafficPackageSearch{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, item := range page.Items {
		if item.PackageNo == input.PackageNo {
			found = true
			if item.OccupiedQuantity != 1 || item.RemainingQuantity == nil || *item.RemainingQuantity != 0 || item.Enabled {
				t.Fatalf("stock ceiling projection: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("unpublished sold package disappeared")
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order WHERE order_no='`+checkout.OrderNo+`' AND status_code='pending_payment'`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order_item WHERE order_no='`+checkout.OrderNo+`' AND product_name_en_us='Test traffic package' AND total_amount_minor=1000`, 1)
	fixture.payInvoice(t, checkout.InvoiceNo)
	if _, err = fixture.settlementService(t).SettleOrder(fixture.ctx, checkout.OrderNo); err != nil {
		t.Fatalf("real paid stock evidence: %+v", err)
	}
	page, err = fixture.service.ListTrafficPackages(fixture.ctx, TrafficPackageSearch{Keyword: input.NameEnUS})
	if err != nil || len(page.Items) != 1 || page.Items[0].OccupiedQuantity != 1 || *page.Items[0].RemainingQuantity != 0 {
		t.Fatalf("paid stock released: %+v / %+v", page, err)
	}
}

func TestTrafficPackageQuoteAndEligibilityPostgreSQL(t *testing.T) {
	fixture := newTrafficPackageFixture(t)
	input := AccessoryQuoteInput{CustomerNo: "CUS_checkout_test", ServiceNo: "SRV_checkout_original", Action: serveroperationresourcecap.ActionTrafficAdd, PackageNo: "POL_checkout_test", IdempotencyKey: "fixed-package-preview"}
	for name, raw := range map[string]string{"capacity": "{\"trafficGb\":999}", "amount": "{\"amountMinor\":1}", "provider": "{\"serverId\":99}"} {
		t.Run(name, func(t *testing.T) {
			altered := input
			altered.Parameters = json.RawMessage(raw)
			if _, err := fixture.service.Preview(fixture.ctx, altered); !bizerr.Is(err, CodeAccessoryPriceInvalid) {
				t.Fatalf("browser parameters accepted: %+v", err)
			}
		})
	}
	quote, err := fixture.service.PreviewCurrentCustomer(fixture.ctx, input)
	if err != nil {
		t.Fatalf("real fixed package preview: %+v", err)
	}
	if quote.TrafficGB != 100 || quote.TotalAmountMinor != 1000 || quote.RequestedQuantity != 1 || quote.PackageNameEnUS != "Test traffic package" {
		t.Fatalf("wrong frozen fixed terms: %+v", quote)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_server_accessory_quote WHERE quote_no='`+quote.QuoteNo+`' AND parameters='{"trafficGb":100}'::jsonb AND total_amount_minor=1000`, 1)
	for _, customer := range []string{"CUS_other", "../../bad"} {
		if _, err = fixture.service.ForServerTrafficPackages(fixture.ctx, customer, input.ServiceNo, TrafficPackageSearch{}); err == nil {
			t.Fatal("foreign service exposed")
		}
	}
	page, err := fixture.service.CustomerTrafficPackages(fixture.ctx, input.ServiceNo, TrafficPackageSearch{})
	if err != nil || page.Total != 1 {
		t.Fatalf("eligible customer packages: %+v / %+v", page, err)
	}
	usd := trafficInput()
	usd.CurrencyCode = "USD"
	usdPackage, err := fixture.service.SaveTrafficPackage(fixture.ctx, usd)
	if err != nil {
		t.Fatal(err)
	}
	page, err = fixture.service.CustomerTrafficPackages(fixture.ctx, input.ServiceNo, TrafficPackageSearch{})
	if err != nil || page.Total != 1 {
		t.Fatalf("wrong-currency package leaked: %+v / %+v", page, err)
	}
	foreignCurrency := input
	foreignCurrency.PackageNo, foreignCurrency.IdempotencyKey = usdPackage.PackageNo, "wrong-currency-preview"
	if _, err = fixture.service.PreviewCurrentCustomer(fixture.ctx, foreignCurrency); !bizerr.Is(err, CodeAccessoryQuoteUnavailable) {
		t.Fatalf("direct wrong-currency selection accepted: %+v", err)
	}
	if _, err = fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us) VALUES ('prd_other','ctg_checkout_test','active','other','其他商品','Other product'); INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code) VALUES ('ver_other','prd_other',1,'draft','manual')`); err != nil {
		t.Fatal(err)
	}
	prepareTrafficProductBinding(t, fixture, "prd_other", "ver_other", "idc-resource", true, true)
	other := trafficInput()
	other.ProductNos = []string{"prd_other"}
	otherPackage, err := fixture.service.SaveTrafficPackage(fixture.ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	page, err = fixture.service.CustomerTrafficPackages(fixture.ctx, input.ServiceNo, TrafficPackageSearch{})
	if err != nil || page.Total != 1 {
		t.Fatalf("wrong-product package leaked: %+v / %+v", page, err)
	}
	foreignProduct := input
	foreignProduct.PackageNo, foreignProduct.IdempotencyKey = otherPackage.PackageNo, "wrong-product-preview"
	if _, err = fixture.service.PreviewCurrentCustomer(fixture.ctx, foreignProduct); !bizerr.Is(err, CodeAccessoryQuoteUnavailable) {
		t.Fatalf("direct wrong-product selection accepted: %+v", err)
	}
	edit := trafficInput()
	edit.PackageNo = input.PackageNo
	edit.ExpectedVersion = 1
	edit.Enabled = false
	if _, err = fixture.service.SaveTrafficPackage(fixture.ctx, edit); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.service.CheckoutCurrentCustomer(fixture.ctx, AccessoryCheckoutInput{QuoteNo: quote.QuoteNo, IdempotencyKey: "unpublished-checkout"}); !bizerr.Is(err, CodeAccessoryPriceConflict) {
		t.Fatalf("unpublished stale quote bought: %+v", err)
	}
	if _, err = fixture.service.Preview(fixture.ctx, input); !bizerr.Is(err, CodeAccessoryQuoteUnavailable) {
		t.Fatalf("unpublished new quote accepted: %+v", err)
	}
	fixture.service.directory = trafficTestResource{time.Now().UTC().Add(-time.Minute)}
	if _, err = fixture.service.CustomerTrafficPackages(fixture.ctx, input.ServiceNo, TrafficPackageSearch{}); !bizerr.Is(err, CodeAccessoryQuoteUnavailable) {
		t.Fatal("expired capability accepted")
	}
}

// TestTrafficPackageCurrentProductBindingPostgreSQL exercises the catalog owner
// projection with current published facts, not the deprecated product registry.
func TestTrafficPackageCurrentProductBindingPostgreSQL(t *testing.T) {
	fixture := newTrafficPackageFixture(t)
	for _, kind := range []string{"server", "ssl", "generic", "manual", "unbound", "history", "no_policy"} {
		prepareTrafficCandidate(t, fixture, kind)
	}
	products, err := fixture.service.TrafficPackageProducts(fixture.ctx, "Traffic candidate", nil)
	if err != nil || len(products) != 1 || products[0].ProductNo != "prd_candidate_server" {
		t.Fatalf("non-server/current binding leaked: %+v / %+v", products, err)
	}
	for _, kind := range []string{"ssl", "generic", "manual", "unbound", "history", "no_policy"} {
		t.Run(kind, func(t *testing.T) {
			input := trafficInput()
			input.ProductNos = []string{"prd_candidate_" + kind}
			if _, err := fixture.service.SaveTrafficPackage(fixture.ctx, input); !bizerr.Is(err, CodeAccessoryPriceInvalid) {
				t.Fatalf("ineligible scope accepted: %+v", err)
			}
		})
	}
	input := trafficInput()
	input.ProductNos = []string{"prd_candidate_server"}
	if _, err = fixture.service.SaveTrafficPackage(fixture.ctx, input); err != nil {
		t.Fatalf("current bound server without legacy entry rejected: %+v", err)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_legacy_server_product`, 0)
	if _, err = fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product SET name_en_us='Traffic candidate_100%' WHERE product_no='prd_candidate_server'`); err != nil {
		t.Fatal(err)
	}
	products, err = fixture.service.TrafficPackageProducts(fixture.ctx, "_100%", nil)
	if err != nil || len(products) != 1 {
		t.Fatalf("literal keyword lost: %+v / %+v", products, err)
	}
	products, err = fixture.service.TrafficPackageProducts(fixture.ctx, "_10%", nil)
	if err != nil || len(products) != 0 {
		t.Fatalf("keyword wildcards interpreted: %+v / %+v", products, err)
	}
}

// prepareTrafficCandidate varies only owner/policy/current-binding evidence.
// All candidates use actual catalog foreign keys and immutable release guards.
func prepareTrafficCandidate(t *testing.T, fixture accessoryCheckoutFixture, kind string) {
	t.Helper()
	productNo, versionNo := "prd_candidate_"+kind, "ver_candidate_"+kind
	if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us)
VALUES (?,'ctg_checkout_test','active',?,'候选商品','Traffic candidate')`, productNo, kind); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code) VALUES (?,?,1,'draft','manual')`, versionNo, productNo); err != nil {
		t.Fatal(err)
	}
	owner, ongoing := "idc-resource", true
	if kind == "ssl" {
		owner = "yunsu-ssl"
	} else if kind == "generic" {
		owner, ongoing = "", false
	}
	if kind != "no_policy" {
		prepareTrafficProductBinding(t, fixture, productNo, versionNo, owner, ongoing, kind != "unbound")
	} else if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product SET current_version_no=? WHERE product_no=?`, versionNo, productNo); err != nil {
		t.Fatal(err)
	}
	if kind == "manual" {
		if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy SET strategy_code='manual',provision_method='',query_method='' WHERE version_no=?`, versionNo); err != nil {
			t.Fatal(err)
		}
	}
	if kind == "history" {
		current := versionNo + "_new"
		if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code) VALUES (?,?,2,'draft','manual')`, current, productNo); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product SET current_version_no=? WHERE product_no=?`, current, productNo); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_catalog_product_version version
SET status_code=CASE WHEN version.version_no=product.current_version_no THEN 'published' ELSE 'superseded' END,published_at=CURRENT_TIMESTAMP
FROM plugin_idc_core_catalog_product product WHERE product.product_no=version.product_no AND product.product_no=?`, productNo); err != nil {
		t.Fatal(err)
	}
}

// cloneTrafficFixtureRow copies a fixture-owned row's ordinary columns only.
// Table and column names come exclusively from these test literals and schema;
// identity IDs are regenerated and every replacement remains a bound value.
func cloneTrafficFixtureRow(t *testing.T, fixture accessoryCheckoutFixture, table, key, value string, changes map[string]any) {
	t.Helper()
	var columns []struct{ ColumnName string }
	if err := fixture.database.GetScan(fixture.ctx, &columns, `SELECT column_name FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=? AND is_identity='NO' ORDER BY ordinal_position`, table); err != nil {
		t.Fatal(err)
	}
	var names, selectors []string
	var args []any
	for _, column := range columns {
		name := `"` + column.ColumnName + `"`
		names = append(names, name)
		if replacement, ok := changes[column.ColumnName]; ok {
			selectors = append(selectors, "?")
			args = append(args, replacement)
		} else {
			selectors = append(selectors, name)
		}
	}
	args = append(args, value)
	if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO "`+table+`" (`+strings.Join(names, ",")+`) SELECT `+strings.Join(selectors, ",")+` FROM "`+table+`" WHERE "`+key+`"=?`, args...); err != nil {
		t.Fatalf("clone synthetic %s: %+v", table, err)
	}
}

func TestTrafficPackageConcurrentCheckoutPostgreSQL(t *testing.T) {
	fixture := newTrafficPackageFixture(t)
	cloneTrafficFixtureRow(t, fixture, "plugin_idc_core_customer_subject", "customer_no", "CUS_checkout_test", map[string]any{"customer_no": "CUS_traffic_second", "customer_code": 10002})
	cloneTrafficFixtureRow(t, fixture, "plugin_idc_core_service_instance", "service_no", "SRV_checkout_original", map[string]any{"service_no": "SRV_traffic_second", "customer_no": "CUS_traffic_second", "runtime_status_code": "running", "current_price_snapshot_no": nil})
	if err := fixture.database.Transaction(fixture.ctx, func(ctx context.Context, _ gdb.TX) error {
		transactionFixture := fixture
		transactionFixture.ctx = ctx
		cloneTrafficFixtureRow(t, transactionFixture, "plugin_idc_core_service_price_snapshot", "service_no", "SRV_checkout_original", map[string]any{"price_snapshot_no": "PRICE_traffic_second", "service_no": "SRV_traffic_second", "subject_no": "SRV_traffic_second", "customer_no": "CUS_traffic_second", "source_quote_no": nil, "source_quote_item_no": nil})
		cloneTrafficFixtureRow(t, transactionFixture, "plugin_idc_core_service_price_component_snapshot", "service_no", "SRV_checkout_original", map[string]any{"component_snapshot_no": "PCM_traffic_second", "price_snapshot_no": "PRICE_traffic_second", "service_no": "SRV_traffic_second", "subject_no": "SRV_traffic_second"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_service_instance SET current_price_snapshot_no='PRICE_traffic_second' WHERE service_no='SRV_traffic_second'`); err != nil {
		t.Fatal(err)
	}
	var inputs []AccessoryCheckoutInput
	customers := []string{"CUS_checkout_test", "CUS_traffic_second"}
	for i, number := range []string{"SRV_checkout_original", "SRV_traffic_second"} {
		quote, err := fixture.service.Preview(fixture.ctx, AccessoryQuoteInput{CustomerNo: customers[i], ServiceNo: number, Action: serveroperationresourcecap.ActionTrafficAdd, PackageNo: "POL_checkout_test", IdempotencyKey: fmt.Sprintf("stock-preview-%d", i)})
		if err != nil {
			t.Fatalf("prepare competing quote: %+v", err)
		}
		inputs = append(inputs, AccessoryCheckoutInput{QuoteNo: quote.QuoteNo, IdempotencyKey: fmt.Sprintf("stock-checkout-%d", i)})
	}
	var successes atomic.Int32
	var failures atomic.Int32
	var wait sync.WaitGroup
	start := make(chan struct{})
	for i, input := range inputs {
		wait.Add(1)
		go func(customerNo string, input AccessoryCheckoutInput) {
			defer wait.Done()
			<-start
			result, err := fixture.service.CheckoutForStaff(fixture.ctx, customerNo, input, 101, PermissionTrafficAdd)
			if err == nil {
				successes.Add(1)
				replay, replayErr := fixture.service.CheckoutForStaff(fixture.ctx, customerNo, input, 101, PermissionTrafficAdd)
				if replayErr != nil || !replay.Reused || replay.OperationNo != result.OperationNo {
					t.Errorf("replay consumed stock: %+v / %+v", replay, replayErr)
				}
			} else if bizerr.Is(err, CodeAccessoryPriceConflict) {
				failures.Add(1)
			} else {
				t.Errorf("unexpected competing error: %+v", err)
			}
		}(customers[i], input)
	}
	close(start)
	wait.Wait()
	if successes.Load() != 1 || failures.Load() != 1 {
		t.Fatalf("one stock accepted %d, conflicts %d", successes.Load(), failures.Load())
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_business_operation WHERE operation_type_code='addon'`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 1)
	page, err := fixture.service.ListTrafficPackages(fixture.ctx, TrafficPackageSearch{})
	if err != nil || len(page.Items) != 1 || page.Items[0].OccupiedQuantity != 1 || *page.Items[0].RemainingQuantity != 0 {
		t.Fatalf("concurrent stock: %+v / %+v", page, err)
	}
	// Paid and unknown operation outcomes still occupy the purchased package.
	for _, status := range []string{"manual_review"} {
		rollback := errors.New("rollback synthetic outcome")
		err = fixture.database.Transaction(fixture.ctx, func(ctx context.Context, _ gdb.TX) error {
			if _, updateErr := fixture.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code=?`, status); updateErr != nil {
				return updateErr
			}
			var count []struct{ Quantity int64 }
			if scanErr := fixture.service.trafficPackageOccupancy(ctx, []string{"POL_checkout_test"}).Scan(&count); scanErr != nil {
				return scanErr
			}
			if len(count) != 1 || count[0].Quantity != 1 {
				t.Errorf("%s released stock", status)
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatalf("stock status %s: %+v", status, err)
		}
	}
}
