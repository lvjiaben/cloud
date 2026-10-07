package contractsnapshot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const contractSnapshotPostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

func TestPrepareOptionalOrderPostgreSQL(t *testing.T) {
	database, cleanup := newContractSnapshotTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	serviceValue, err := New(database, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatalf("construct contract snapshot service: %v", err)
	}
	service := serviceValue.(*serviceImpl)

	snapshot, err := service.PrepareOptionalOrder(ctx, "ORDER_OPTIONAL_OK")
	if err != nil {
		t.Fatalf("prepare optional fulfilled order: %v", err)
	}
	if snapshot.RequiresContract || snapshot.StatusCode != "fulfilled" ||
		snapshot.Template.TemplateNo != "" || len(snapshot.Items) != 1 ||
		snapshot.Items[0].ContractProductName != "" ||
		snapshot.Items[0].ProductNameEnUS != "Phone" ||
		snapshot.Items[0].Quantity != 3 ||
		!snapshot.Items[0].RequiresContract {
		t.Fatalf("unexpected optional order snapshot: %+v", snapshot)
	}
	snapshot.Clear()
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_commerce_order_item_quantity_snapshot
WHERE tenant_id = 0 AND order_no = 'ORDER_OPTIONAL_OK'`); err != nil {
		t.Fatalf("remove authoritative quantity snapshot: %v", err)
	}
	if rejected, rejectErr := service.PrepareOptionalOrder(ctx, "ORDER_OPTIONAL_OK"); rejectErr == nil {
		rejected.Clear()
		t.Fatal("order without an authoritative quantity snapshot was accepted")
	}

	for _, orderNo := range []string{"ORDER_CANCELLED", "ORDER_REFUNDED"} {
		if rejected, rejectErr := service.PrepareOptionalOrder(ctx, orderNo); rejectErr == nil {
			rejected.Clear()
			t.Fatalf("ineligible order %s was accepted", orderNo)
		}
	}
}

func newContractSnapshotTestDatabase(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(contractSnapshotPostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run contract snapshot PostgreSQL tests", contractSnapshotPostgreSQLDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open contract snapshot test admin database: %v", err)
	}
	var randomValue [8]byte
	if _, err = rand.Read(randomValue[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_contract_snapshot_test_" + hex.EncodeToString(randomValue[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	scopedDSN := dsn + "?search_path=" + schema
	if strings.Contains(dsn, "?") {
		scopedDSN = dsn + "&search_path=" + schema
	}
	database, err := gdb.New(gdb.ConfigNode{Link: scopedDSN})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, contractSnapshotBaseSchema); err != nil {
		t.Fatalf("create contract snapshot base schema: %v", err)
	}
	if _, err = database.Exec(ctx, contractSnapshotFixtures); err != nil {
		t.Fatalf("seed contract snapshot fixtures: %v", err)
	}
	cleanup := func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close contract snapshot database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_contract_snapshot_test_") {
			t.Errorf("refusing unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); dropErr != nil {
			t.Errorf("drop contract snapshot schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close contract snapshot admin database: %v", closeErr)
		}
	}
	return database, cleanup
}

const contractSnapshotBaseSchema = `
CREATE TABLE plugin_idc_core_customer_subject (
    tenant_id INT NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    customer_type_code VARCHAR(24) NOT NULL
);
CREATE TABLE plugin_idc_core_customer_individual_profile (
    tenant_id INT NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    legal_name VARCHAR(256) NOT NULL,
    identity_lookup_digest CHAR(64) NOT NULL,
    identity_case_no VARCHAR(40) NOT NULL,
    identity_status_code VARCHAR(24) NOT NULL,
    identity_verified_at TIMESTAMPTZ NOT NULL,
    identity_expires_at TIMESTAMPTZ,
    version BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_customer_contact (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    contact_no VARCHAR(40) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    phone_country_code VARCHAR(8) NOT NULL,
    phone VARCHAR(32) NOT NULL,
    phone_verified_at TIMESTAMPTZ,
    version BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_customer_contact_role (
    tenant_id INT NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    contact_no VARCHAR(40) NOT NULL,
    role_code VARCHAR(24) NOT NULL,
    status_code VARCHAR(24) NOT NULL
);
CREATE TABLE plugin_idc_core_merchant_profile (
    tenant_id INT NOT NULL,
    merchant_no VARCHAR(40) NOT NULL,
    legal_name VARCHAR(256) NOT NULL,
    country_code CHAR(2) NOT NULL,
    enabled BOOLEAN NOT NULL,
    version BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order (
    tenant_id INT NOT NULL,
    order_no VARCHAR(40) NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    total_amount_minor BIGINT NOT NULL,
    requires_contract BOOLEAN NOT NULL,
    version BIGINT NOT NULL,
    contract_template_no VARCHAR(64) NOT NULL DEFAULT '',
    contract_template_version_no BIGINT NOT NULL DEFAULT 0,
    contract_template_name VARCHAR(200) NOT NULL DEFAULT '',
    contract_language_code VARCHAR(8) NOT NULL DEFAULT '',
    contract_scope_code VARCHAR(24) NOT NULL DEFAULT '',
    contract_signing_order_code VARCHAR(24) NOT NULL DEFAULT ''
);
CREATE TABLE plugin_idc_core_billing_invoice (
    tenant_id INT NOT NULL,
    order_no VARCHAR(40) NOT NULL,
    source_code VARCHAR(24) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    paid_amount_minor BIGINT NOT NULL,
    refunded_amount_minor BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item (
    tenant_id INT NOT NULL,
    order_no VARCHAR(40) NOT NULL,
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    line_number INT NOT NULL,
    order_item_no VARCHAR(40) NOT NULL,
    product_no VARCHAR(40) NOT NULL,
    product_name_zh_cn VARCHAR(200) NOT NULL,
    product_name_en_us VARCHAR(200) NOT NULL,
    server_quantity INT NOT NULL,
    purchase_cycles INT NOT NULL,
    period_unit_code VARCHAR(24) NOT NULL,
    period_count INT NOT NULL,
    requires_contract BOOLEAN NOT NULL,
    total_amount_minor BIGINT NOT NULL
);
CREATE TABLE plugin_idc_core_commerce_order_item_quantity_snapshot (
    tenant_id INT NOT NULL,
    order_item_no VARCHAR(40) NOT NULL,
    order_no VARCHAR(40) NOT NULL,
    quantity INT NOT NULL
);`

const contractSnapshotFixtures = `
INSERT INTO plugin_idc_core_customer_subject
    (tenant_id, customer_no, customer_type_code)
VALUES (0, 'CUSTOMER_OPTIONAL', 'individual');
INSERT INTO plugin_idc_core_customer_individual_profile
    (tenant_id, customer_no, legal_name, identity_lookup_digest, identity_case_no,
     identity_status_code, identity_verified_at, version)
VALUES
    (0, 'CUSTOMER_OPTIONAL', 'Example Customer',
     'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
     'CASE_OPTIONAL', 'verified', CURRENT_TIMESTAMP, 1);
INSERT INTO plugin_idc_core_customer_contact
    (tenant_id, customer_no, contact_no, status_code, phone_country_code,
     phone, phone_verified_at, version)
VALUES
    (0, 'CUSTOMER_OPTIONAL', 'CONTACT_OPTIONAL', 'active', '+86',
     '13800138000', CURRENT_TIMESTAMP, 1);
INSERT INTO plugin_idc_core_customer_contact_role
    (tenant_id, customer_no, contact_no, role_code, status_code)
VALUES
    (0, 'CUSTOMER_OPTIONAL', 'CONTACT_OPTIONAL', 'primary', 'active');
INSERT INTO plugin_idc_core_merchant_profile
    (tenant_id, merchant_no, legal_name, country_code, enabled, version)
VALUES (0, 'MERCHANT_LOCAL', 'Local IDC Merchant', 'CN', TRUE, 1);
INSERT INTO plugin_idc_core_commerce_order
    (tenant_id, order_no, customer_no, status_code, currency_code,
     total_amount_minor, requires_contract, version)
VALUES
    (0, 'ORDER_OPTIONAL_OK', 'CUSTOMER_OPTIONAL', 'fulfilled', 'CNY', 100, FALSE, 2),
    (0, 'ORDER_CANCELLED', 'CUSTOMER_OPTIONAL', 'cancelled', 'CNY', 100, FALSE, 2),
    (0, 'ORDER_REFUNDED', 'CUSTOMER_OPTIONAL', 'refunded', 'CNY', 100, FALSE, 2);
INSERT INTO plugin_idc_core_billing_invoice
    (tenant_id, order_no, source_code, status_code, paid_amount_minor, refunded_amount_minor)
VALUES
    (0, 'ORDER_OPTIONAL_OK', 'order', 'paid', 100, 0),
    (0, 'ORDER_CANCELLED', 'order', 'cancelled', 0, 0),
    (0, 'ORDER_REFUNDED', 'order', 'refunded', 100, 100);
INSERT INTO plugin_idc_core_commerce_order_item
    (tenant_id, order_no, line_number, order_item_no, product_no,
     product_name_zh_cn, product_name_en_us, server_quantity, purchase_cycles,
     period_unit_code, period_count, requires_contract, total_amount_minor)
VALUES
    (0, 'ORDER_OPTIONAL_OK', 1, 'ITEM_OPTIONAL_OK', 'PRODUCT_PHONE',
     '手机', 'Phone', 99, 1, 'month', 12, TRUE, 100),
    (0, 'ORDER_CANCELLED', 1, 'ITEM_CANCELLED', 'PRODUCT_SERVER',
     '独立服务器', 'Dedicated Server', 1, 1, 'month', 12, TRUE, 100),
    (0, 'ORDER_REFUNDED', 1, 'ITEM_REFUNDED', 'PRODUCT_SERVER',
     '独立服务器', 'Dedicated Server', 1, 1, 'month', 12, TRUE, 100);
INSERT INTO plugin_idc_core_commerce_order_item_quantity_snapshot
    (tenant_id, order_item_no, order_no, quantity)
VALUES
    (0, 'ITEM_OPTIONAL_OK', 'ORDER_OPTIONAL_OK', 3),
    (0, 'ITEM_CANCELLED', 'ORDER_CANCELLED', 1),
    (0, 'ITEM_REFUNDED', 'ORDER_REFUNDED', 1);`
