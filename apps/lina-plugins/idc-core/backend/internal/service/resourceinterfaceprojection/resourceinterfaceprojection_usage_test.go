// This file verifies local usage against isolated PostgreSQL data, including
// historical bindings, imported services, soft-deleted products, and denial.

package resourceinterfaceprojection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

// usageTestAuthorizer records the exact permission without granting writes.
type usageTestAuthorizer struct {
	denied bool
	calls  int
}

func (auth *usageTestAuthorizer) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	auth.calls++
	if auth.denied || request.Permission != "idc:admin:resource:configuration:list" || request.TargetNo != "provider-configurations" {
		return 0, gerror.New("denied")
	}
	return 7, nil
}

func TestUsageProjectionPostgreSQL(t *testing.T) {
	ctx, database := newUsageDatabase(t)
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_instance(tenant_id,resource_owner_code,channel_no,entitlement_status_code,runtime_status_code,source_code) VALUES
(0,'idc-resource','CHN_A','active','running','order'),
(0,'idc-resource','CHN_A','active','stopped','manual'),
(0,'idc-resource','CHN_A','active','suspended','migration'),
(0,'idc-resource','CHN_A','terminated','terminated','order'),
(0,'idc-resource','CHN_A','pending_activation','provisioning','order'),
(0,'idc-resource','CHN_B','active','running','order'),
(0,'idc-resource','demo_resource_channel_bt','active','running','manual'),
(1,'idc-resource','CHN_A','active','running','order'),
(0,'other-owner','CHN_A','active','running','order');
INSERT INTO plugin_idc_core_product_automation_interface_instance(tenant_id,interface_instance_no,resource_owner_id,resource_channel_ref) VALUES (0,'interface_a','idc-resource','CHN_A'),(0,'interface_b','idc-resource','CHN_B'),(1,'other_tenant','idc-resource','CHN_A');
INSERT INTO plugin_idc_core_catalog_product_version VALUES
(0,'current_a','product_a'),(0,'history_a','product_a'),(0,'draft_a','draft_only'),(0,'current_b','moved_to_b'),(0,'deleted_a','removed'),(1,'other_version','other_product');
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding VALUES
(0,'current_a','interface_a'),(0,'history_a','interface_a'),(0,'draft_a','interface_a'),(0,'current_b','interface_b'),(0,'deleted_a','interface_a'),(1,'other_version','other_tenant');
INSERT INTO plugin_idc_core_catalog_product(tenant_id,product_no,current_version_no,deleted_at) VALUES
(0,'product_a','current_a',NULL),(0,'moved_to_b','current_b',NULL),(0,'draft_only',NULL,NULL),(0,'removed','deleted_a',CURRENT_TIMESTAMP),(1,'other_product','other_version',NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	auth := &usageTestAuthorizer{}
	reader, err := NewUsageReader(database, auth)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reader.BatchGet(ctx, []string{"CHN_A", "CHN_B", "CHN_EMPTY"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ActiveInstances != 3 || items[0].TotalInstances != 5 || items[0].ConfiguredProducts != 1 || items[1].ConfiguredProducts != 1 || items[2].TotalInstances != 0 {
		t.Fatalf("unexpected usage: %+v", items)
	}
	assertUsageIndexPaths(t, ctx, database)
	// The directory contains owner-valid opaque names as well as generated CHN_
	// numbers. One such row must not take down the whole configuration list.
	opaque, err := reader.BatchGet(ctx, []string{"CHN_A", "demo_resource_channel_bt", "provider.channel-2"})
	if err != nil || len(opaque) != 3 || opaque[1].ActiveInstances != 1 || opaque[1].TotalInstances != 1 || opaque[2].TotalInstances != 0 {
		t.Fatalf("opaque owner references: %+v, err=%v", opaque, err)
	}
	auth.denied = true
	if result, deniedErr := reader.BatchGet(ctx, []string{"CHN_A"}); deniedErr == nil || result != nil {
		t.Fatalf("denial leaked result=%+v err=%v", result, deniedErr)
	}
	auth.denied = false
	for _, channels := range [][]string{{"CHN_A", "CHN_A"}, {"bad channel"}, {""}, {"1channel"}, {strings.Repeat("a", 257)}, make([]string, 101)} {
		if _, invalidErr := reader.BatchGet(ctx, channels); invalidErr == nil {
			t.Fatalf("accepted invalid input %+v", channels)
		}
	}
	if empty, emptyErr := reader.BatchGet(ctx, nil); emptyErr != nil || len(empty) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, emptyErr)
	}
}

// TestUsageChannelContract keeps read-only references aligned with the owner,
// including length boundaries, while excluding SQL fragments and control text.
func TestUsageChannelContract(t *testing.T) {
	for _, value := range []string{"CHN_A", "demo_resource_channel_bt", "provider.channel-2", "a", strings.Repeat("a", 256)} {
		if !usageChannelPattern.MatchString(value) {
			t.Errorf("rejected owner-valid reference %q", value)
		}
	}
	for _, value := range []string{"", "1channel", " bad", "bad channel", "bad\n", "x'; DROP TABLE x;--", strings.Repeat("a", 257)} {
		if usageChannelPattern.MatchString(value) {
			t.Errorf("accepted invalid reference %q", value)
		}
	}
}

// assertUsageIndexPaths proves index eligibility without claiming a production
// latency benchmark; sequential scans are disabled only inside this test TX.
func assertUsageIndexPaths(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	err := database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(txCtx, "SET LOCAL enable_seqscan=off"); err != nil {
			return err
		}
		cases := []struct {
			query string
			index string
		}{
			{`EXPLAIN SELECT channel_no,COUNT(*) FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND resource_owner_code='idc-resource' AND channel_no='CHN_A' GROUP BY channel_no`, "usage_service_channel"},
			{`EXPLAIN SELECT interface.resource_channel_ref,COUNT(DISTINCT product.product_no)
			 FROM plugin_idc_core_catalog_product product
			 JOIN plugin_idc_core_catalog_product_version version ON version.tenant_id=product.tenant_id AND version.product_no=product.product_no AND version.version_no=product.current_version_no
			 JOIN plugin_idc_core_catalog_product_version_fulfillment_binding binding ON binding.tenant_id=version.tenant_id AND binding.version_no=version.version_no
			 JOIN plugin_idc_core_product_automation_interface_instance interface ON interface.tenant_id=binding.tenant_id AND interface.interface_instance_no=binding.interface_instance_no
			 WHERE product.tenant_id=0 AND product.deleted_at IS NULL AND interface.resource_owner_id='idc-resource' AND interface.interface_origin_code='resource' AND interface.resource_channel_ref='CHN_A'
			 GROUP BY interface.resource_channel_ref`, "idx_idc_product_automation_resource_projection"},
		}
		for _, item := range cases {
			rows, err := database.GetArray(txCtx, item.query)
			if err != nil {
				return err
			}
			var plan strings.Builder
			for _, row := range rows {
				plan.WriteString(row.String())
				plan.WriteByte('\n')
			}
			if !strings.Contains(plan.String(), item.index) {
				t.Errorf("expected index %s in plan:\n%s", item.index, plan.String())
			}
			t.Logf("index path %s:\n%s", item.index, plan.String())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// newUsageDatabase owns and cleans every table; it never reads the live schema.
func newUsageDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN for isolated PostgreSQL")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_interface_usage_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA `+quoteIdentifier(schema)+` CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_service_instance(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,resource_owner_code TEXT,channel_no TEXT,entitlement_status_code TEXT,runtime_status_code TEXT,source_code TEXT);
CREATE INDEX usage_service_channel ON plugin_idc_core_service_instance(tenant_id,resource_owner_code,channel_no);
CREATE TABLE plugin_idc_core_product_automation_interface_instance(tenant_id INT,interface_instance_no TEXT UNIQUE,resource_owner_id TEXT,resource_channel_ref TEXT,interface_origin_code TEXT DEFAULT 'resource');
CREATE INDEX idx_idc_product_automation_resource_projection ON plugin_idc_core_product_automation_interface_instance(tenant_id,resource_owner_id,resource_channel_ref) WHERE interface_origin_code='resource';
CREATE TABLE plugin_idc_core_catalog_product_version(tenant_id INT,version_no TEXT UNIQUE,product_no TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id INT,version_no TEXT,interface_instance_no TEXT);
CREATE INDEX usage_product_binding ON plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id,interface_instance_no,version_no);
CREATE TABLE plugin_idc_core_catalog_product(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,product_no TEXT UNIQUE,current_version_no TEXT,deleted_at TIMESTAMPTZ);`); err != nil {
		t.Fatal(err)
	}
	return ctx, database
}
