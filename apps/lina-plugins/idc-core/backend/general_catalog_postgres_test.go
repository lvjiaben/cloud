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

const catalogIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"
const catalogCodegenDSNEnv = "LINAPRO_IDC_CORE_CODEGEN_DSN"

func newCatalogIntegrationSchema(t *testing.T) string {
	t.Helper()
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	return "linapro_catalog_test_" + hex.EncodeToString(value[:])
}

func quoteCatalogIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendCatalogDSNParameter(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

const catalogDictionaryFixtureSQL = `
CREATE TABLE sys_dict_type (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    CONSTRAINT uk_catalog_test_dict_type UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
    dict_type VARCHAR(128) NOT NULL DEFAULT '', label VARCHAR(128) NOT NULL DEFAULT '',
    value VARCHAR(128) NOT NULL DEFAULT '', sort INT NOT NULL DEFAULT 0,
    tag_style VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    CONSTRAINT uk_catalog_test_dict_data UNIQUE (tenant_id, dict_type, value)
);`

func TestGeneralCatalogMigrationPreservesLegacyEvidencePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("idc_general_catalog_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); _ = admin.Close(ctx) })
	scoped, err := gdb.New(gdb.ConfigNode{Link: appendGeneralCatalogDSN(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scoped.Close(ctx) })
	_, err = scoped.Exec(ctx, `
CREATE TABLE sys_dict_type (tenant_id BIGINT, type TEXT, PRIMARY KEY (tenant_id,type));
CREATE TABLE sys_dict_data (tenant_id BIGINT, dict_type TEXT, value TEXT, PRIMARY KEY (tenant_id,dict_type,value));
CREATE TABLE plugin_idc_core_catalog_category (
 tenant_id BIGINT NOT NULL, category_no VARCHAR(64) PRIMARY KEY,
 parent_category_no VARCHAR(64),
 category_type_code VARCHAR(24) NOT NULL,
 name_zh_cn TEXT NOT NULL, name_en_us TEXT NOT NULL
);
CREATE TABLE plugin_idc_core_catalog_product (
 tenant_id BIGINT NOT NULL, product_no VARCHAR(64) PRIMARY KEY,
 category_no VARCHAR(64) NOT NULL, product_type_code VARCHAR(24) NOT NULL,
 configuration_mode_code VARCHAR(24) NOT NULL, current_version_no VARCHAR(64)
);
CREATE TABLE plugin_idc_core_catalog_product_version (
 tenant_id BIGINT NOT NULL, version_no VARCHAR(64) PRIMARY KEY, product_no VARCHAR(64) NOT NULL
);
CREATE TABLE plugin_idc_core_marketing_campaign_scope (
 tenant_id BIGINT NOT NULL, campaign_no VARCHAR(64) NOT NULL,
 scope_kind_code VARCHAR(24) NOT NULL, match_mode_code VARCHAR(16) NOT NULL,
 reference_code VARCHAR(128) NOT NULL,
 UNIQUE (campaign_no,scope_kind_code,match_mode_code,reference_code)
);`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 19; i++ {
		kind := "cloud_server"
		if i%2 == 0 {
			kind = "bare_metal"
		}
		if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_category
(tenant_id,category_no,category_type_code,name_zh_cn,name_en_us) VALUES (0,?,?,?,?)`,
			fmt.Sprintf("category-%02d", i), kind, fmt.Sprintf("分类%d", i), fmt.Sprintf("Category %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = scoped.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product
(tenant_id,product_no,category_no,product_type_code,configuration_mode_code,current_version_no)
VALUES (0,'product-01','category-01','cloud_server','configurable','version-01');
INSERT INTO plugin_idc_core_catalog_product_version (tenant_id,version_no,product_no)
VALUES (0,'version-01','product-01')`); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, `
UPDATE plugin_idc_core_catalog_category SET parent_category_no='category-01' WHERE category_no='category-02';
INSERT INTO plugin_idc_core_marketing_campaign_scope
(tenant_id,campaign_no,scope_kind_code,match_mode_code,reference_code)
VALUES (0,'campaign-01','category','include','category-01');
INSERT INTO sys_dict_type (tenant_id,type) VALUES (0,'idc_catalog_category_type'),(7,'idc_catalog_category_type');
INSERT INTO sys_dict_data (tenant_id,dict_type,value) VALUES
(0,'idc_catalog_category_type','cloud_server'),(7,'idc_catalog_category_type','bare_metal');`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "073-general-product-catalog.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("migration is not idempotent: %v", err)
	}
	var counts struct {
		Categories       int `orm:"categories"`
		Products         int `orm:"products"`
		Versions         int `orm:"versions"`
		CategoryEvidence int `orm:"category_evidence"`
		ProductEvidence  int `orm:"product_evidence"`
		PointerMatches   int `orm:"pointer_matches"`
		NumberMappings   int `orm:"number_mappings"`
		InvalidNumbers   int `orm:"invalid_numbers"`
		ReferenceMatches int `orm:"reference_matches"`
		LegacyDictRows   int `orm:"legacy_dict_rows"`
	}
	err = scoped.GetScan(ctx, &counts, `SELECT
(SELECT count(*) FROM plugin_idc_core_catalog_category) categories,
(SELECT count(*) FROM plugin_idc_core_catalog_product) products,
(SELECT count(*) FROM plugin_idc_core_catalog_product_version) versions,
(SELECT count(*) FROM plugin_idc_core_legacy_server_category) category_evidence,
(SELECT count(*) FROM plugin_idc_core_legacy_server_product) product_evidence,
(SELECT count(*) FROM plugin_idc_core_catalog_category_no_migration) number_mappings,
(SELECT count(*) FROM plugin_idc_core_catalog_category WHERE category_no !~ '^ctg_[0-9a-f]{32}$' OR (parent_category_no IS NOT NULL AND parent_category_no !~ '^ctg_[0-9a-f]{32}$')) invalid_numbers,
(SELECT count(*) FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_category c ON c.category_no=p.category_no JOIN plugin_idc_core_marketing_campaign_scope s ON s.scope_kind_code='category' AND s.reference_code=c.category_no) reference_matches,
(SELECT count(*) FROM sys_dict_type WHERE type='idc_catalog_category_type') + (SELECT count(*) FROM sys_dict_data WHERE dict_type='idc_catalog_category_type') legacy_dict_rows,
(SELECT count(*) FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_product_version v ON v.version_no=p.current_version_no AND v.product_no=p.product_no) pointer_matches`)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Categories != 19 || counts.Products != 1 || counts.Versions != 1 || counts.CategoryEvidence != 19 || counts.ProductEvidence != 1 || counts.PointerMatches != 1 || counts.NumberMappings != 19 || counts.InvalidNumbers != 0 || counts.ReferenceMatches != 1 || counts.LegacyDictRows != 0 {
		t.Fatalf("migration lost catalog data: %+v", counts)
	}
	for _, removed := range []struct{ table, column string }{
		{"plugin_idc_core_catalog_category", "category_type_code"},
		{"plugin_idc_core_catalog_product", "product_type_code"},
		{"plugin_idc_core_catalog_product", "configuration_mode_code"},
	} {
		countValue, queryErr := scoped.GetValue(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=? AND table_name=? AND column_name=?`, schema, removed.table, removed.column)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if countValue.Int() != 0 {
			t.Fatalf("generic catalog still exposes %s", removed.column)
		}
	}
}

func appendGeneralCatalogDSN(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
