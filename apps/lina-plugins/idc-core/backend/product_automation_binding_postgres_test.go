// This file verifies the PostgreSQL data contract for generic automation
// interface instances and immutable single-interface product-version bindings.

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestProductAutomationBindingMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}

	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_product_automation_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteProductAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA `+quoteProductAutomationIdentifier(schema)+` CASCADE`); cleanupErr != nil {
			t.Errorf("drop product automation test schema: %v", cleanupErr)
		}
		if cleanupErr := admin.Close(ctx); cleanupErr != nil {
			t.Errorf("close product automation admin database: %v", cleanupErr)
		}
	})

	scoped, err := gdb.New(gdb.ConfigNode{Link: appendProductAutomationDSN(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanupErr := scoped.Close(ctx); cleanupErr != nil {
			t.Errorf("close product automation scoped database: %v", cleanupErr)
		}
	})

	_, err = scoped.Exec(ctx, `
CREATE TABLE plugin_idc_core_catalog_product_version (
    tenant_id INT NOT NULL DEFAULT 0, version_no VARCHAR(40) NOT NULL,
    status_code VARCHAR(16) NOT NULL, PRIMARY KEY (version_no),
    UNIQUE (tenant_id, version_no)
);
INSERT INTO plugin_idc_core_catalog_product_version (version_no, status_code) VALUES
    ('ver_11111111111111111111111111111111', 'draft'),
    ('ver_22222222222222222222222222222222', 'draft'),
    ('ver_33333333333333333333333333333333', 'draft'),
    ('ver_44444444444444444444444444444444', 'draft'),
    ('ver_55555555555555555555555555555555', 'draft');`)
	if err != nil {
		t.Fatal(err)
	}

	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "074-product-automation-binding.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}

	insertInterface := `INSERT INTO plugin_idc_core_product_automation_interface_instance
(interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,created_by,changed_by)
VALUES (?,?,?,?,?,?,?,?,?,?,?)`
	if _, err = scoped.Exec(ctx, insertInterface,
		"pfi_11111111111111111111111111111111", "魔方云 A 接口", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101, 101); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, insertInterface,
		"pfi_22222222222222222222222222222222", "魔方云 B 接口", "backup-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("2", 64), strings.Repeat("b", 64), 101, 101); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, insertInterface,
		"pfi_33333333333333333333333333333333", "格式化方法接口", "method-automation",
		"product-fulfillment", "2026.8.0", "v1", `{query.status,read_v2,retry-task}`, strings.Repeat("3", 64), strings.Repeat("c", 64), 101, 101); err != nil {
		t.Fatalf("accept host-compatible method names: %v", err)
	}
	if _, err = scoped.Exec(ctx, insertInterface,
		"pfi_99999999999999999999999999999999", "误建接口", "mistaken-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("9", 64), strings.Repeat("d", 64), 101, 101); err != nil {
		t.Fatalf("insert never-referenced mistaken interface: %v", err)
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no='pfi_99999999999999999999999999999999'`); err != nil {
		t.Fatalf("delete never-referenced mistaken interface: %v", err)
	}
	assertProductAutomationRejected(t, scoped, ctx, "an incompatible extension point", insertInterface,
		"pfi_44444444444444444444444444444444", "短信接口", "server-automation",
		"sms-channel", "2026.8.0", "v1", `{send}`, strings.Repeat("4", 64), strings.Repeat("d", 64), 101, 101)
	assertProductAutomationRejected(t, scoped, ctx, "a non-normalized method set", insertInterface,
		"pfi_55555555555555555555555555555555", "乱序方法接口", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{query,provision,query}`, strings.Repeat("5", 64), strings.Repeat("e", 64), 101, 101)
	for index, methods := range []string{`{Provision}`, `{1query}`, `{query..status}`, `{query status}`} {
		assertProductAutomationRejected(t, scoped, ctx, "a method outside the host identifier contract", insertInterface,
			fmt.Sprintf("pfi_%032x", index+6), "非法方法接口", "server-automation",
			"product-fulfillment", "2026.8.0", "v1", methods, strings.Repeat("6", 64), strings.Repeat("f", 64), 101, 101)
	}
	insertInterfaceWithTenant := `INSERT INTO plugin_idc_core_product_automation_interface_instance
(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,created_by,changed_by)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`
	assertProductAutomationRejected(t, scoped, ctx, "a tenant-scoped interface", insertInterfaceWithTenant,
		1, "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "租户接口", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("7", 64), strings.Repeat("a", 64), 101, 101)
	assertProductAutomationRejected(t, scoped, ctx, "an anonymous interface creator", insertInterface,
		"pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "匿名接口", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("7", 64), strings.Repeat("a", 64), 0, 101)
	assertProductAutomationRejected(t, scoped, ctx, "an invalid interface identifier", insertInterface,
		"interface-a", "错误编号接口", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("7", 64), strings.Repeat("a", 64), 101, 101)
	assertProductAutomationRejected(t, scoped, ctx, "a blank interface display name", insertInterface,
		"pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("7", 64), strings.Repeat("a", 64), 101, 101)
	assertProductAutomationRejected(t, scoped, ctx, "a padded interface display name", insertInterface,
		"pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", " 接口名称 ", "server-automation",
		"product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("7", 64), strings.Repeat("a", 64), 101, 101)
	assertProductAutomationRejected(t, scoped, ctx, "interface instance mutation",
		`UPDATE plugin_idc_core_product_automation_interface_instance
SET display_name='已改名' WHERE interface_instance_no='pfi_11111111111111111111111111111111'`)
	assertProductAutomationRejected(t, scoped, ctx, "an anonymous state changer",
		`UPDATE plugin_idc_core_product_automation_interface_instance
SET new_business_state='stopped',version=2,changed_by=0,changed_at=CURRENT_TIMESTAMP
WHERE interface_instance_no='pfi_33333333333333333333333333333333'`)
	assertProductAutomationRejected(t, scoped, ctx, "a state change without a version increment",
		`UPDATE plugin_idc_core_product_automation_interface_instance
SET new_business_state='stopped',changed_by=101,changed_at=CURRENT_TIMESTAMP
WHERE interface_instance_no='pfi_33333333333333333333333333333333'`)
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_instance
SET new_business_state='stopped',version=2,changed_by=101,changed_at=CURRENT_TIMESTAMP
WHERE interface_instance_no='pfi_33333333333333333333333333333333'`); err != nil {
		t.Fatalf("accept constrained stop transition: %v", err)
	}
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_instance
SET new_business_state='accepting',version=3,changed_by=102,changed_at=CURRENT_TIMESTAMP
WHERE interface_instance_no='pfi_33333333333333333333333333333333'`); err != nil {
		t.Fatalf("accept constrained resume transition: %v", err)
	}

	insertBinding := `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
(version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by)
VALUES (?,?,?,?,?,?,?,?,?,?,?)`
	if _, err = scoped.Exec(ctx, insertBinding,
		"ver_11111111111111111111111111111111", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101); err != nil {
		t.Fatal(err)
	}

	assertProductAutomationRejected(t, scoped, ctx, "a second interface for one version", insertBinding,
		"ver_11111111111111111111111111111111", "pfi_22222222222222222222222222222222",
		"魔方云 B 接口", "backup-automation", "product-fulfillment", "2026.8.0", "v1", `{}`, strings.Repeat("2", 64), strings.Repeat("b", 64), 101)
	assertProductAutomationRejected(t, scoped, ctx, "an incompatible frozen contract", insertBinding,
		"ver_22222222222222222222222222222222", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v2", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101)
	insertBindingWithTenant := `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
(tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`
	assertProductAutomationRejected(t, scoped, ctx, "a tenant-scoped binding", insertBindingWithTenant,
		1, "ver_22222222222222222222222222222222", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101)
	assertProductAutomationRejected(t, scoped, ctx, "an anonymous binding actor", insertBinding,
		"ver_22222222222222222222222222222222", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 0)
	if _, err = scoped.Exec(ctx, insertBinding,
		"ver_33333333333333333333333333333333", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101); err != nil {
		t.Fatal(err)
	}
	for name, statement := range map[string]string{
		"binding id": `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET id=id+100 WHERE version_no='ver_33333333333333333333333333333333'`,
		"binding tenant": `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET tenant_id=1 WHERE version_no='ver_33333333333333333333333333333333'`,
		"bound version": `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET version_no='ver_22222222222222222222222222222222'
WHERE version_no='ver_33333333333333333333333333333333'`,
	} {
		assertProductAutomationRejected(t, scoped, ctx, "draft "+name+" reassignment", statement)
	}
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET interface_instance_no='pfi_22222222222222222222222222222222',
    interface_display_name_snapshot='魔方云 B 接口', plugin_id='backup-automation',
    authorized_methods=ARRAY[]::TEXT[], release_digest=?, declaration_digest=?
WHERE version_no='ver_33333333333333333333333333333333'`, strings.Repeat("2", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("replace draft binding: %v", err)
	}
	if _, err = scoped.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_product_version_fulfillment_binding
WHERE version_no='ver_33333333333333333333333333333333'`); err != nil {
		t.Fatalf("remove draft binding: %v", err)
	}

	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version
SET status_code='published' WHERE version_no='ver_11111111111111111111111111111111'`); err != nil {
		t.Fatal(err)
	}
	assertProductAutomationRejected(t, scoped, ctx, "supplier replacement on a published version",
		`UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
		SET interface_instance_no='pfi_22222222222222222222222222222222'
WHERE version_no='ver_11111111111111111111111111111111'`)
	assertProductAutomationRejected(t, scoped, ctx, "published binding deletion",
		`DELETE FROM plugin_idc_core_catalog_product_version_fulfillment_binding
WHERE version_no='ver_11111111111111111111111111111111'`)

	frozenCount, err := scoped.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_core_catalog_product_version_fulfillment_binding
WHERE version_no='ver_11111111111111111111111111111111'
  AND plugin_id='server-automation'
  AND plugin_release_version='2026.8.0'
  AND contract_version='v1'
  AND interface_instance_no='pfi_11111111111111111111111111111111'
  AND interface_display_name_snapshot='魔方云 A 接口'
  AND authorized_methods=ARRAY['provision','query']::TEXT[]
  AND release_digest='1111111111111111111111111111111111111111111111111111111111111111'
  AND declaration_digest='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`)
	if err != nil {
		t.Fatal(err)
	}
	if frozenCount.Int() != 1 {
		t.Fatalf("published binding snapshot was not preserved: %d", frozenCount.Int())
	}

	contender, err := gdb.New(gdb.ConfigNode{Link: appendProductAutomationDSN(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanupErr := contender.Close(ctx); cleanupErr != nil {
			t.Errorf("close product automation contender database: %v", cleanupErr)
		}
	})
	assertPublicationExcludesBindingWrite(t, scoped, contender, ctx,
		"ver_44444444444444444444444444444444", insertBinding,
		"ver_44444444444444444444444444444444", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101)
	if _, err = scoped.Exec(ctx, insertBinding,
		"ver_55555555555555555555555555555555", "pfi_11111111111111111111111111111111",
		"魔方云 A 接口", "server-automation", "product-fulfillment", "2026.8.0", "v1", `{provision,query}`, strings.Repeat("1", 64), strings.Repeat("a", 64), 101); err != nil {
		t.Fatal(err)
	}
	assertPublicationExcludesBindingWrite(t, scoped, contender, ctx,
		"ver_55555555555555555555555555555555",
		`UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding
SET interface_instance_no='pfi_22222222222222222222222222222222',
    interface_display_name_snapshot='魔方云 B 接口', plugin_id='backup-automation',
    authorized_methods=ARRAY[]::TEXT[], release_digest=?, declaration_digest=?
WHERE version_no='ver_55555555555555555555555555555555'`, strings.Repeat("2", 64), strings.Repeat("b", 64))
}

func TestProductAutomationBindingMigrationUpgradesOld074PostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newCatalogIntegrationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteProductAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+quoteProductAutomationIdentifier(schema)+` CASCADE`)
		_ = admin.Close(ctx)
	})
	scoped, err := gdb.New(gdb.ConfigNode{Link: appendProductAutomationDSN(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scoped.Close(ctx) })
	_, err = scoped.Exec(ctx, `
CREATE TABLE plugin_idc_core_catalog_product_version (
    tenant_id INT NOT NULL DEFAULT 0, version_no VARCHAR(40) NOT NULL PRIMARY KEY,
    status_code VARCHAR(16) NOT NULL, UNIQUE (tenant_id, version_no)
);
CREATE TABLE plugin_idc_core_product_automation_interface_instance (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    interface_instance_no VARCHAR(40) NOT NULL UNIQUE,
    display_name VARCHAR(128) NOT NULL,
    plugin_id VARCHAR(64) NOT NULL,
    extension_point_code VARCHAR(64) NOT NULL,
    plugin_release_version VARCHAR(32) NOT NULL,
    contract_version VARCHAR(32) NOT NULL,
    authorized_methods TEXT[] NOT NULL,
    release_digest CHAR(64) NOT NULL,
    declaration_digest CHAR(64) NOT NULL,
    created_by INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, interface_instance_no)
);
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_automation_interface_update()
RETURNS TRIGGER AS $$ BEGIN
    RAISE EXCEPTION 'old 074 rejects every interface update';
END; $$ LANGUAGE plpgsql;
CREATE TRIGGER trg_idc_guard_automation_interface_update
BEFORE UPDATE OR DELETE ON plugin_idc_core_product_automation_interface_instance
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_automation_interface_update();
INSERT INTO plugin_idc_core_product_automation_interface_instance
(interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,
 contract_version,authorized_methods,release_digest,declaration_digest,created_by)
VALUES ('pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','旧接口','server-automation','product-fulfillment',
 '2026.8.0','v1',ARRAY[]::TEXT[],repeat('1',64),repeat('a',64),101);`)
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "074-product-automation-binding.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("upgrade old 074 schema with installed immutable trigger: %v", err)
	}
	preserved, err := scoped.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_core_product_automation_interface_instance
WHERE interface_instance_no='pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
  AND display_name='旧接口' AND new_business_state='accepting'
  AND version=1 AND created_by=101 AND changed_by=101`)
	if err != nil || preserved.Int() != 1 {
		t.Fatalf("old interface was not preserved and backfilled: count=%d err=%v", preserved.Int(), err)
	}
}

func TestProductAutomationBindingMigrationAfterGeneralCatalogPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newCatalogIntegrationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteProductAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA `+quoteProductAutomationIdentifier(schema)+` CASCADE`); cleanupErr != nil {
			t.Errorf("drop product automation migration-chain schema: %v", cleanupErr)
		}
		if cleanupErr := admin.Close(ctx); cleanupErr != nil {
			t.Errorf("close product automation migration-chain admin database: %v", cleanupErr)
		}
	})
	scoped, err := gdb.New(gdb.ConfigNode{Link: appendProductAutomationDSN(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanupErr := scoped.Close(ctx); cleanupErr != nil {
			t.Errorf("close product automation migration-chain database: %v", cleanupErr)
		}
	})
	if _, err = scoped.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create catalog dictionary fixture: %v", err)
	}
	_, file, _, _ := runtime.Caller(0)
	migrationRoot := filepath.Join(filepath.Dir(file), "..", "manifest", "sql")
	for _, migrationName := range []string{
		"010-catalog-foundation.sql",
		"073-general-product-catalog.sql",
		"074-product-automation-binding.sql",
		"075-structured-product-configuration.sql",
	} {
		migration, readErr := os.ReadFile(filepath.Join(migrationRoot, migrationName))
		if readErr != nil {
			t.Fatalf("read migration %s: %v", migrationName, readErr)
		}
		migrationSQL := string(migration)
		if migrationName == "075-structured-product-configuration.sql" {
			marker := strings.Index(migrationSQL, "-- Generic transaction evidence is a sidecar")
			if marker < 0 {
				t.Fatal("structured configuration sidecar migration marker is missing")
			}
			migrationSQL = migrationSQL[:marker]
		}
		if _, err = scoped.Exec(ctx, migrationSQL); err != nil {
			t.Fatalf("execute migration %s: %v", migrationName, err)
		}
		if migrationName == "010-catalog-foundation.sql" {
			if _, err = scoped.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_category
(category_no,name_zh_cn,name_en_us) VALUES
('ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','升级分类','Upgrade Category');
INSERT INTO plugin_idc_core_catalog_product
(product_no,category_no,public_slug,name_zh_cn,name_en_us) VALUES
('prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','upgrade-product','升级商品','Upgrade Product');
INSERT INTO plugin_idc_core_catalog_product_version
(version_no,product_no,version_number) VALUES
('ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1);`); err != nil {
				t.Fatalf("seed pre-073 catalog row: %v", err)
			}
		}
	}
	for _, tableName := range []string{
		"plugin_idc_core_catalog_product_version",
		"plugin_idc_core_product_automation_interface_instance",
		"plugin_idc_core_catalog_product_version_fulfillment_binding",
		"plugin_idc_core_product_automation_interface_configuration_revision",
		"plugin_idc_core_product_automation_interface_secret_reference",
		"plugin_idc_core_catalog_product_version_configuration",
	} {
		count, queryErr := scoped.GetValue(ctx, `SELECT count(*) FROM information_schema.tables
WHERE table_schema=current_schema() AND table_name=?`, tableName)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if count.Int() != 1 {
			t.Fatalf("migration chain did not create %s", tableName)
		}
	}
	preservedVersionCount, err := scoped.GetValue(ctx, `SELECT count(*)
FROM plugin_idc_core_catalog_product_version
WHERE version_no='ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' AND status_code='draft'`)
	if err != nil {
		t.Fatal(err)
	}
	if preservedVersionCount.Int() != 1 {
		t.Fatal("073 to 074 upgrade did not preserve the existing draft product version")
	}
	configurationMigration, err := os.ReadFile(filepath.Join(migrationRoot, "075-structured-product-configuration.sql"))
	if err != nil {
		t.Fatal(err)
	}
	configurationSQL := string(configurationMigration)
	marker := strings.Index(configurationSQL, "-- Generic transaction evidence is a sidecar")
	if marker < 0 {
		t.Fatal("structured configuration sidecar migration marker is missing")
	}
	if _, err = scoped.Exec(ctx, configurationSQL[:marker]); err != nil {
		t.Fatalf("reapply structured configuration migration: %v", err)
	}
	if _, err = scoped.Exec(ctx, `
INSERT INTO plugin_idc_core_product_automation_interface_instance
(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,
 plugin_release_version,contract_version,authorized_methods,release_digest,
 declaration_digest,created_by,changed_by)
VALUES
(0,'pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','SSL A','ssl-provider','product-fulfillment',
 'v1.0.0','v1',ARRAY[]::TEXT[],repeat('a',64),repeat('b',64),1,1);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
(tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,
 plugin_id,extension_point_code,plugin_release_version,contract_version,
 authorized_methods,release_digest,declaration_digest,bound_by)
VALUES
(0,'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb','SSL A',
 'ssl-provider','product-fulfillment','v1.0.0','v1',ARRAY[]::TEXT[],repeat('a',64),repeat('b',64),1);
INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision
(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,
 public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by)
VALUES
(0,'pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1,'v1','{"version":"v1","fields":[]}'::jsonb,
 '{}'::jsonb,repeat('c',64),repeat('d',64),1,1)
RETURNING id`); err != nil {
		t.Fatalf("seed structured configuration contract: %v", err)
	}
	revisionID, err := scoped.GetValue(ctx, `SELECT id
FROM plugin_idc_core_product_automation_interface_configuration_revision
WHERE interface_instance_no='pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb' AND revision=1`)
	if err != nil || revisionID.Int64() <= 0 {
		t.Fatalf("read interface configuration revision: id=%d err=%v", revisionID.Int64(), err)
	}
	if _, err = scoped.Exec(ctx, `
INSERT INTO plugin_idc_core_product_automation_interface_secret_reference
(tenant_id,configuration_revision_id,field_key,secret_ref,configured,masked_tail,secret_updated_at)
VALUES (0,?,'api_token','secret://idc-core/configuration/test',TRUE,'****test',CURRENT_TIMESTAMP)`, revisionID.Int64()); err != nil {
		t.Fatalf("insert secretcap reference: %v", err)
	}
	if _, err = scoped.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_product_version_configuration
(tenant_id,version_no,interface_instance_no,interface_configuration_revision,
 product_schema_version,product_declaration_snapshot,merchant_value_snapshot,
 customer_schema_version,customer_declaration_snapshot,declaration_digest,
 merchant_value_digest,changed_by)
VALUES
(0,'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',1,
 'v1','{"version":"v1","fields":[]}'::jsonb,'{}'::jsonb,
 'v1','{"version":"v1","fields":[]}'::jsonb,repeat('e',64),repeat('f',64),1)`); err != nil {
		t.Fatalf("insert product-version configuration snapshot: %v", err)
	}
	assertProductAutomationRejected(t, scoped, ctx, "non-contiguous interface configuration revision", `
INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision
(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,
 public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by)
VALUES
(0,'pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',3,'v1','{}'::jsonb,'{}'::jsonb,
 repeat('1',64),repeat('2',64),0,1)`)
	if _, err = scoped.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version
SET status_code='published',published_at=CURRENT_TIMESTAMP
WHERE version_no='ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`); err != nil {
		t.Fatal(err)
	}
	assertProductAutomationRejected(t, scoped, ctx, "published configuration mutation", `
UPDATE plugin_idc_core_catalog_product_version_configuration
SET merchant_value_snapshot='{"changed":true}'::jsonb,version=version+1,changed_by=1
WHERE version_no='ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'`)
}

func assertPublicationExcludesBindingWrite(
	t *testing.T,
	publisher gdb.DB,
	contender gdb.DB,
	ctx context.Context,
	versionNo string,
	statement string,
	args ...any,
) {
	t.Helper()
	tx, err := publisher.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	t.Cleanup(func() {
		if !committed {
			_ = tx.Rollback()
		}
	})
	if _, err = tx.Exec(`UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?`, versionNo); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		_, writeErr := contender.Exec(ctx, statement, args...)
		result <- writeErr
	}()
	<-started
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case writeErr := <-result:
		t.Fatalf("binding write returned before the publication lock was released: %v", writeErr)
	case <-timer.C:
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	committed = true
	select {
	case writeErr := <-result:
		if writeErr == nil {
			t.Fatal("publication and the concurrent binding write both succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent binding write did not finish after publication committed")
	}
}

func assertProductAutomationRejected(t *testing.T, database gdb.DB, ctx context.Context, reason, statement string, args ...any) {
	t.Helper()
	if _, err := database.Exec(ctx, statement, args...); err == nil {
		t.Fatalf("expected PostgreSQL to reject %s", reason)
	}
}

func quoteProductAutomationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendProductAutomationDSN(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%s%s", dsn, separator, parameter)
}
