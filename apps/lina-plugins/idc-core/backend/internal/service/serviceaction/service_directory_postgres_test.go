// This file verifies paid customer, directory and plugin filters on real owner projections.
package serviceaction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
)

type directoryProjectionRegistry struct {
	pluginhost.BusinessExtensionRegistrar
	batches int
}

func (r *directoryProjectionRegistry) GetProvidersExact(_ context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	r.batches++
	results := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for i := range inputs {
		results[i] = &capmodel.BusinessExtensionProviderEvidence{AuthorizedMethods: []string{"provision", "query"}, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: "product-fulfillment-contract", Version: "v1", Values: map[string]any{"capability": map[string]any{"enUS": "Managed product", "zhCN": "受管商品"}, "granularity": "per_order_item", "ongoingManagement": true, "renewalSupport": "unsupported", "expiryManagement": "provider", "entitlementOwnerId": "idc-core", "methodRoles": map[string]any{"provision": "provision", "query": "query"}}}}
	}
	return results, nil
}

func TestDirectoryProjectionPaidCustomerPluginScopePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	schema := "managed_directory_" + hex.EncodeToString(suffix[:])
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteServiceActionIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := admin.Exec(context.Background(), `DROP SCHEMA `+quoteServiceActionIdentifier(schema)+` CASCADE`); e != nil {
			t.Error(e)
		}
		if e := admin.Close(context.Background()); e != nil {
			t.Error(e)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendServiceActionSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := db.Close(context.Background()); e != nil {
			t.Error(e)
		}
	})
	_, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_service_entitlement(id BIGSERIAL,tenant_id INT DEFAULT 0,entitlement_no TEXT,customer_no TEXT,product_no TEXT,source_intent_no TEXT,order_no TEXT,plugin_id TEXT,plugin_release_version TEXT DEFAULT 'v1',contract_version TEXT DEFAULT 'v1',declaration_digest TEXT DEFAULT 'declaration',status_code TEXT DEFAULT 'active',started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,expires_at TIMESTAMPTZ,updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT DEFAULT 0,intent_no TEXT,configuration_snapshot_no TEXT DEFAULT 'snapshot',product_name_zh_cn TEXT DEFAULT '商品',product_name_en_us TEXT DEFAULT 'Product',release_digest TEXT DEFAULT 'release');
CREATE TABLE plugin_idc_core_commerce_structured_configuration_snapshot(tenant_id INT DEFAULT 0,snapshot_no TEXT,typed_value_snapshot JSONB DEFAULT '{}');
CREATE TABLE plugin_idc_core_customer_subject(tenant_id INT DEFAULT 0,customer_no TEXT,display_name TEXT);
CREATE TABLE plugin_idc_core_managed_service_action_task(tenant_id INT DEFAULT 0,id BIGSERIAL,entitlement_no TEXT,safe_projection JSONB);
CREATE TABLE plugin_idc_core_commerce_order(tenant_id INT DEFAULT 0,order_no TEXT,customer_no TEXT,paid_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_product_directory_membership(tenant_id INT DEFAULT 0,product_no TEXT,directory_no TEXT);
INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no) VALUES('snapshot');
INSERT INTO plugin_idc_core_customer_subject(customer_no,display_name) VALUES('customer-a','A'),('customer-b','B');
INSERT INTO plugin_idc_core_product_directory_membership(product_no,directory_no) VALUES('product-a','menu-a'),('product-b','menu-b');
INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,product_no,source_intent_no,order_no,plugin_id,status_code) VALUES
('paid-a','customer-a','product-a','intent-a','order-a','ssl','pending_activation'),
('unpaid-a','customer-a','product-a','intent-u','order-u','ssl','active'),
('paid-other-menu','customer-a','product-b','intent-b','order-b','ssl','active'),
('paid-other-plugin','customer-a','product-a','intent-d','order-d','domain','active'),
('foreign-customer','customer-b','product-a','intent-f','order-f','ssl','active'),
('foreign-payment','customer-a','product-a','intent-x','order-x','ssl','active');
INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no) SELECT source_intent_no FROM plugin_idc_core_service_entitlement;
INSERT INTO plugin_idc_core_commerce_order(order_no,customer_no,paid_at) SELECT order_no,CASE WHEN entitlement_no='foreign-payment' THEN 'customer-b' ELSE customer_no END,CASE WHEN entitlement_no='unpaid-a' THEN NULL ELSE CURRENT_TIMESTAMP END FROM plugin_idc_core_service_entitlement;`)
	if err != nil {
		t.Fatal(err)
	}
	registry := &directoryProjectionRegistry{}
	reader, err := New(db, registry)
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{Audience: AudienceCustomer, CustomerNo: "customer-a", UserID: 7}
	rows, _, err := reader.ListEntitlements(ctx, actor, 0, 100, DirectoryFilter{DirectoryNo: "menu-a", PluginID: "ssl"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].EntitlementNo != "paid-a" || rows[0].Status != "pending_activation" {
		t.Fatalf("paid pending projection=%+v", rows)
	}
	if registry.batches != 1 {
		t.Fatalf("expected one exact declaration batch, got %d", registry.batches)
	}
	rows, _, err = reader.ListEntitlements(ctx, actor, 0, 100)
	if err != nil || len(rows) != 3 {
		t.Fatalf("all paid rows=%+v err=%v", rows, err)
	}
	rows, _, err = reader.ListEntitlements(ctx, actor, 0, 100, DirectoryFilter{DirectoryNo: "unknown", PluginID: "ssl"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("unknown scope broadened access: %+v %v", rows, err)
	}
}
