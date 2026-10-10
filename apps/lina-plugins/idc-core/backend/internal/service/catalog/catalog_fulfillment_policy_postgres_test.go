package catalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/dao"
)

func TestFulfillmentPolicyPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, `
CREATE TABLE plugin_idc_core_catalog_product_version (
 tenant_id INT NOT NULL DEFAULT 0, version_no VARCHAR(40) PRIMARY KEY, status_code VARCHAR(16) NOT NULL
);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_binding (
 tenant_id INT NOT NULL DEFAULT 0, version_no VARCHAR(40) PRIMARY KEY, declaration_digest CHAR(64) NOT NULL
);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy (
 tenant_id INT NOT NULL DEFAULT 0, version_no VARCHAR(40) PRIMARY KEY,
 strategy_code VARCHAR(32) NOT NULL, granularity_code VARCHAR(32) NOT NULL,
 ongoing_management BOOLEAN NOT NULL DEFAULT FALSE,
 renewal_mode_code VARCHAR(32) NOT NULL DEFAULT 'unavailable', renewal_method VARCHAR(128) NOT NULL DEFAULT '', entitlement_owner_id VARCHAR(64) NOT NULL DEFAULT '',
 provision_method VARCHAR(128) NOT NULL DEFAULT '', query_method VARCHAR(128) NOT NULL DEFAULT '',
 maximum_attempts INT NOT NULL, retry_base_seconds INT NOT NULL, policy_digest CHAR(64) NOT NULL,
 created_by BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO plugin_idc_core_catalog_product_version(version_no,status_code) VALUES
 ('ver_11111111111111111111111111111111','draft'),
 ('ver_22222222222222222222222222222222','draft'),
 ('ver_33333333333333333333333333333333','draft');`); err != nil {
		t.Fatal(err)
	}

	manual, err := SaveFulfillmentPolicy(ctx, FulfillmentPolicyUpdateInput{
		VersionNo: "ver_11111111111111111111111111111111", StrategyCode: FulfillmentStrategyManual,
	}, FulfillmentPolicyWriteAuthorization{ActorUserID: 7})
	if err != nil || manual.GranularityCode != productfulfillmentcap.GranularityPerOrderItem {
		t.Fatalf("save unbound manual policy: policy=%+v err=%v", manual, err)
	}
	storedManual, err := loadFulfillmentPolicyEvidence(ctx, "ver_11111111111111111111111111111111")
	if err != nil || storedManual == nil || *storedManual != *manual ||
		storedManual.MaximumAttempts != 10 || storedManual.RetryBaseSeconds != 30 {
		t.Fatalf("read manual policy: stored=%+v err=%v", storedManual, err)
	}
	if _, err = SaveFulfillmentPolicy(ctx, FulfillmentPolicyUpdateInput{
		VersionNo: "ver_22222222222222222222222222222222", StrategyCode: FulfillmentStrategyApprovalThenPlugin,
	}, FulfillmentPolicyWriteAuthorization{ActorUserID: 7}); !bizerr.Is(err, CodeFulfillmentPolicyBindingRequired) {
		t.Fatalf("unbound plugin policy returned %v", err)
	}

	digest := strings.Repeat("a", 64)
	contract := &FrozenFulfillmentContract{
		Capability:         &capmodel.LocalizedText{ZhCN: "证书签发", EnUS: "Certificate issuance"},
		Granularity:        productfulfillmentcap.GranularityPerQuantityUnit,
		OngoingManagement:  true,
		RenewalMode:        productfulfillmentcap.RenewalModeLocalEntitlement,
		EntitlementOwnerID: "ssl-owner",
		ProvisionMethod:    "certificate.issue", QueryMethod: "certificate.query", DeclarationDigest: digest,
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
 (tenant_id,version_no,declaration_digest) VALUES(0,?,?)`, "ver_22222222222222222222222222222222", digest); err != nil {
		t.Fatal(err)
	}
	pluginPolicy, err := SaveFulfillmentPolicy(ctx, FulfillmentPolicyUpdateInput{
		VersionNo: "ver_22222222222222222222222222222222", StrategyCode: FulfillmentStrategyPaymentConfirmedPlugin,
	}, FulfillmentPolicyWriteAuthorization{ActorUserID: 7, Contract: contract})
	if err != nil || pluginPolicy.GranularityCode != productfulfillmentcap.GranularityPerQuantityUnit || !pluginPolicy.OngoingManagement ||
		pluginPolicy.ProvisionMethod != "certificate.issue" || pluginPolicy.QueryMethod != "certificate.query" {
		t.Fatalf("save exact plugin policy: policy=%+v err=%v", pluginPolicy, err)
	}
	drifted := *contract
	drifted.DeclarationDigest = strings.Repeat("b", 64)
	if _, err = SaveFulfillmentPolicy(ctx, FulfillmentPolicyUpdateInput{
		VersionNo: "ver_22222222222222222222222222222222", StrategyCode: FulfillmentStrategyPaymentConfirmedPlugin,
	}, FulfillmentPolicyWriteAuthorization{ActorUserID: 7, Contract: &drifted}); !bizerr.Is(err, CodeFulfillmentPolicyContractChanged) {
		t.Fatalf("drifted exact contract returned %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published'
 WHERE version_no='ver_11111111111111111111111111111111'`); err != nil {
		t.Fatal(err)
	}
	if _, err = SaveFulfillmentPolicy(ctx, FulfillmentPolicyUpdateInput{
		VersionNo: "ver_11111111111111111111111111111111", StrategyCode: FulfillmentStrategyManual,
	}, FulfillmentPolicyWriteAuthorization{ActorUserID: 7}); !bizerr.Is(err, CodeFulfillmentPolicyImmutable) {
		t.Fatalf("published policy mutation returned %v", err)
	}
}

func configureFulfillmentPolicyTestDatabase(t *testing.T, ctx context.Context, dsn string) gdb.DB {
	t.Helper()
	original := gdb.GetAllConfig()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 8)
	if _, err = rand.Read(buffer); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_fulfillment_policy_test_" + hex.EncodeToString(buffer)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	scopedDSN := dsn
	separator := "?"
	if strings.Contains(scopedDSN, "?") {
		separator = "&"
	}
	scopedDSN += separator + "search_path=" + url.QueryEscape(schema)
	if err = gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{{Link: scopedDSN}}}); err != nil {
		t.Fatal(err)
	}
	database, err := gdb.Instance()
	if err != nil {
		t.Fatal(err)
	}
	// These tests replace the default group's search_path with distinct schemas
	// and deliberately use different table shapes. GoFrame caches column metadata
	// by group/schema/table, so invalidate the previous fixture's metadata before
	// constructing this fixture rather than silently dropping legitimate DO fields.
	if err = dao.CatalogProductVersion.DB().GetCore().ClearTableFieldsAll(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if database != nil {
			_ = database.Close(ctx)
		}
		_ = gdb.SetConfig(original)
		if strings.HasPrefix(schema, "linapro_fulfillment_policy_test_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	return database
}
