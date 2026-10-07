// This file verifies that the idc-core migration preserves PostgreSQL object
// identities while removing names shortened before the old brand prefix was
// deleted. The fixtures run in isolated schemas and are safe to replay.

package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

type generatedPrimaryKeyName struct {
	tableName   string
	legacyName  string
	currentName string
}

type generatedSequenceName struct {
	tableName   string
	legacyName  string
	currentName string
}

func TestGeneratedObjectNamesAreConsolidatedPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(constraintTriggerMigrationDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx, scopedDB := newGeneratedObjectTestDatabase(t, dsn)
	primaryKeys := generatedPrimaryKeyFixtures()
	sequences := generatedSequenceFixtures()
	tables := make(map[string]struct{}, len(primaryKeys)+len(sequences))
	for _, fixture := range primaryKeys {
		tables[fixture.tableName] = struct{}{}
	}
	for _, fixture := range sequences {
		tables[fixture.tableName] = struct{}{}
	}
	for tableName := range tables {
		if _, err := scopedDB.Exec(ctx, fmt.Sprintf(
			"CREATE TABLE %s (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, evidence TEXT NOT NULL)",
			quoteConstraintTriggerIdentifier(tableName),
		)); err != nil {
			t.Fatalf("create generated-name fixture table %s: %v", tableName, err)
		}
		if _, err := scopedDB.Exec(ctx, fmt.Sprintf(
			"INSERT INTO %s (evidence) VALUES ('preserved')",
			quoteConstraintTriggerIdentifier(tableName),
		)); err != nil {
			t.Fatalf("insert generated-name fixture %s: %v", tableName, err)
		}
	}

	primaryKeyOIDs := make(map[string]int64, len(primaryKeys))
	for _, fixture := range primaryKeys {
		oid, err := scopedDB.GetValue(ctx, `SELECT conindid FROM pg_constraint
WHERE conrelid = to_regclass(?) AND conname = ? AND contype = 'p'`,
			fixture.tableName, fixture.currentName)
		if err != nil || oid.Int64() == 0 {
			t.Fatalf("read fresh primary key %s: oid=%d err=%v", fixture.currentName, oid.Int64(), err)
		}
		primaryKeyOIDs[fixture.currentName] = oid.Int64()
		if _, err = scopedDB.Exec(ctx, fmt.Sprintf(
			"ALTER TABLE %s RENAME CONSTRAINT %s TO %s",
			quoteConstraintTriggerIdentifier(fixture.tableName),
			quoteConstraintTriggerIdentifier(fixture.currentName),
			quoteConstraintTriggerIdentifier(fixture.legacyName),
		)); err != nil {
			t.Fatalf("install legacy primary key name %s: %v", fixture.legacyName, err)
		}
	}

	sequenceOIDs := make(map[string]int64, len(sequences))
	for _, fixture := range sequences {
		oid, err := scopedDB.GetValue(ctx, "SELECT to_regclass(?)::oid", fixture.currentName)
		if err != nil || oid.Int64() == 0 {
			t.Fatalf("read fresh identity sequence %s: oid=%d err=%v", fixture.currentName, oid.Int64(), err)
		}
		sequenceOIDs[fixture.currentName] = oid.Int64()
		if _, err = scopedDB.Exec(ctx, fmt.Sprintf(
			"ALTER SEQUENCE %s RENAME TO %s",
			quoteConstraintTriggerIdentifier(fixture.currentName),
			quoteConstraintTriggerIdentifier(fixture.legacyName),
		)); err != nil {
			t.Fatalf("install legacy identity sequence name %s: %v", fixture.legacyName, err)
		}
	}

	migration, err := os.ReadFile(filepath.Join(
		filepath.Dir(auditEvidenceSealingMigrationPath(t)),
		"074-product-automation-binding.sql",
	))
	if err != nil {
		t.Fatalf("read generated-name migration: %v", err)
	}
	block := taggedMigrationBlock(t, migration, "generated_object_names")
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = scopedDB.Exec(ctx, block); err != nil {
			t.Fatalf("replay generated-name cleanup attempt %d: %v", attempt+1, err)
		}
	}

	for _, fixture := range primaryKeys {
		assertGeneratedObjectName(t, ctx, scopedDB, fixture.legacyName,
			fixture.currentName, primaryKeyOIDs[fixture.currentName])
	}
	for _, fixture := range sequences {
		assertGeneratedObjectName(t, ctx, scopedDB, fixture.legacyName,
			fixture.currentName, sequenceOIDs[fixture.currentName])
	}
	for tableName := range tables {
		evidence, err := scopedDB.GetValue(ctx, fmt.Sprintf(
			"SELECT evidence FROM %s", quoteConstraintTriggerIdentifier(tableName)))
		if err != nil || evidence.String() != "preserved" {
			t.Fatalf("fixture evidence changed for %s: value=%q err=%v",
				tableName, evidence.String(), err)
		}
	}

	rogue := sequences[0]
	if _, err = scopedDB.Exec(ctx, "CREATE SEQUENCE "+quoteConstraintTriggerIdentifier(rogue.legacyName)); err != nil {
		t.Fatalf("create ambiguous legacy sequence: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, block); err == nil {
		t.Fatal("generated-name cleanup accepted simultaneous old and current sequences")
	}
}

func TestLegacyGuardFunctionsAreRenamedWithoutChangingTriggerOIDsPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(constraintTriggerMigrationDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	testCases := []struct {
		name          string
		migration     string
		blockTag      string
		legacyName    string
		currentName   string
		triggerTables []string
		triggerNames  []string
	}{
		{
			name:        "accessory history",
			migration:   "058-server-accessory-pricing.sql",
			blockTag:    "accessory_history_function",
			legacyName:  "plugin_idc_reject_accessory_pricing_history_mutation",
			currentName: "plugin_idc_core_reject_accessory_pricing_history_mutation",
			triggerTables: []string{
				"plugin_idc_core_server_accessory_price_policy_version",
				"plugin_idc_core_server_accessory_quote",
			},
			triggerNames: []string{
				"trg_idc_server_accessory_policy_version_immutable",
				"trg_idc_server_accessory_quote_immutable",
			},
		},
		{
			name:        "IP-change history",
			migration:   "064-server-ip-change.sql",
			blockTag:    "ip_change_history_function",
			legacyName:  "plugin_idc_reject_server_ip_change_history_mutation",
			currentName: "plugin_idc_core_reject_server_ip_change_history_mutation",
			triggerTables: []string{
				"plugin_idc_core_server_ip_change_policy_version",
				"plugin_idc_core_server_ip_change_quote",
				"plugin_idc_core_server_ip_change_event",
			},
			triggerNames: []string{
				"trg_idc_server_ip_change_policy_version_immutable",
				"trg_idc_server_ip_change_quote_immutable",
				"trg_idc_server_ip_change_event_immutable",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verifyLegacyGuardFunctionRename(t, dsn, testCase.migration,
				testCase.blockTag, testCase.legacyName, testCase.currentName,
				testCase.triggerTables, testCase.triggerNames)
		})
	}
}

func newGeneratedObjectTestDatabase(t *testing.T, dsn string) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open generated-name database: %v", err)
	}
	databaseName, err := adminDB.GetValue(ctx, "SELECT current_database()")
	if err != nil || databaseName.String() != "linapro_idc_codegen" {
		_ = adminDB.Close(ctx)
		t.Fatalf("refusing generated-name test database %q: %v", databaseName.String(), err)
	}
	schema := newConstraintTriggerTestSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteConstraintTriggerIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create generated-name schema: %v", err)
	}
	t.Cleanup(func() {
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteConstraintTriggerIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop generated-name schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close generated-name database: %v", closeErr)
		}
	})
	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendConstraintTriggerDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped generated-name database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := scopedDB.Close(ctx); closeErr != nil {
			t.Errorf("close scoped generated-name database: %v", closeErr)
		}
	})
	return ctx, scopedDB
}

func assertGeneratedObjectName(
	t *testing.T,
	ctx context.Context,
	db gdb.DB,
	legacyName string,
	currentName string,
	expectedOID int64,
) {
	t.Helper()
	legacyOID, err := db.GetValue(ctx, "SELECT to_regclass(?)::oid", legacyName)
	if err != nil || legacyOID.Int64() != 0 {
		t.Fatalf("legacy object %s remains: oid=%d err=%v", legacyName, legacyOID.Int64(), err)
	}
	currentOID, err := db.GetValue(ctx, "SELECT to_regclass(?)::oid", currentName)
	if err != nil || currentOID.Int64() != expectedOID {
		t.Fatalf("current object %s changed identity: got=%d want=%d err=%v",
			currentName, currentOID.Int64(), expectedOID, err)
	}
}

func verifyLegacyGuardFunctionRename(
	t *testing.T,
	dsn string,
	migration string,
	blockTag string,
	legacyName string,
	currentName string,
	tables []string,
	triggers []string,
) {
	t.Helper()
	ctx, scopedDB := newGeneratedObjectTestDatabase(t, dsn)
	for _, tableName := range tables {
		if _, err := scopedDB.Exec(ctx, "CREATE TABLE "+quoteConstraintTriggerIdentifier(tableName)+" (id BIGINT)"); err != nil {
			t.Fatalf("create function fixture table %s: %v", tableName, err)
		}
	}
	if _, err := scopedDB.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS TRIGGER
LANGUAGE plpgsql AS $$ BEGIN RETURN OLD; END; $$`, quoteConstraintTriggerIdentifier(legacyName))); err != nil {
		t.Fatalf("create legacy function %s: %v", legacyName, err)
	}
	legacyOID, err := scopedDB.GetValue(ctx, "SELECT to_regprocedure(?)::oid", legacyName+"()")
	if err != nil || legacyOID.Int64() == 0 {
		t.Fatalf("read legacy function %s: oid=%d err=%v", legacyName, legacyOID.Int64(), err)
	}
	for index, tableName := range tables {
		if _, err = scopedDB.Exec(ctx, fmt.Sprintf(
			"CREATE TRIGGER %s BEFORE UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			quoteConstraintTriggerIdentifier(triggers[index]),
			quoteConstraintTriggerIdentifier(tableName),
			quoteConstraintTriggerIdentifier(legacyName),
		)); err != nil {
			t.Fatalf("create legacy function trigger %s: %v", triggers[index], err)
		}
	}
	migrationSQL, err := os.ReadFile(filepath.Join(
		filepath.Dir(auditEvidenceSealingMigrationPath(t)), migration))
	if err != nil {
		t.Fatalf("read function migration %s: %v", migration, err)
	}
	block := taggedMigrationBlock(t, migrationSQL, blockTag)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = scopedDB.Exec(ctx, block); err != nil {
			t.Fatalf("replay function rename %s attempt %d: %v", currentName, attempt+1, err)
		}
	}
	oldOID, err := scopedDB.GetValue(ctx, "SELECT to_regprocedure(?)::oid", legacyName+"()")
	if err != nil || oldOID.Int64() != 0 {
		t.Fatalf("legacy function %s remains: oid=%d err=%v", legacyName, oldOID.Int64(), err)
	}
	newOID, err := scopedDB.GetValue(ctx, "SELECT to_regprocedure(?)::oid", currentName+"()")
	if err != nil || newOID.Int64() != legacyOID.Int64() {
		t.Fatalf("current function %s changed identity: got=%d want=%d err=%v",
			currentName, newOID.Int64(), legacyOID.Int64(), err)
	}
	boundTriggers, err := scopedDB.GetValue(ctx, `SELECT count(*) FROM pg_trigger
WHERE NOT tgisinternal AND tgfoid = to_regprocedure(?)`, currentName+"()")
	if err != nil || boundTriggers.Int() != len(triggers) {
		t.Fatalf("current function %s trigger bindings changed: got=%d want=%d err=%v",
			currentName, boundTriggers.Int(), len(triggers), err)
	}
}

func generatedPrimaryKeyFixtures() []generatedPrimaryKeyName {
	return []generatedPrimaryKeyName{
		{"plugin_idc_core_billing_invoice_transform_destination", "plugin_idc_core_billing_invoice_transform_destinat_pkey", "plugin_idc_core_billing_invoice_transform_destination_pkey"},
		{"plugin_idc_core_commerce_assisted_quote_request_item", "plugin_idc_core_commerce_assisted_quote_request_it_pkey", "plugin_idc_core_commerce_assisted_quote_request_item_pkey"},
		{"plugin_idc_core_commerce_configuration_snapshot_item", "plugin_idc_core_commerce_configuration_snapshot_it_pkey", "plugin_idc_core_commerce_configuration_snapshot_item_pkey"},
		{"plugin_idc_core_marketing_campaign_newcomer_request", "plugin_idc_core_marketing_campaign_newcomer_reques_pkey", "plugin_idc_core_marketing_campaign_newcomer_request_pkey"},
		{"plugin_idc_core_server_accessory_price_policy_version", "plugin_idc_core_server_accessory_price_policy_vers_pkey", "plugin_idc_core_server_accessory_price_policy_version_pkey"},
		{"plugin_idc_core_support_ticket_custom_field_value_option", "plugin_idc_core_support_ticket_custom_field_value__pkey", "plugin_idc_core_support_ticket_custom_field_value_option_pkey"},
	}
}

func generatedSequenceFixtures() []generatedSequenceName {
	return []generatedSequenceName{
		{"plugin_idc_core_billing_invoice_transform_destination", "plugin_idc_core_billing_invoice_transform_destin_id_seq", "plugin_idc_core_billing_invoice_transform_destination_id_seq"},
		{"plugin_idc_core_billing_invoice_waiver_allocation", "plugin_idc_core_billing_invoice_waiver_allocatio_id_seq", "plugin_idc_core_billing_invoice_waiver_allocation_id_seq"},
		{"plugin_idc_core_commerce_assisted_quote_request_item", "plugin_idc_core_commerce_assisted_quote_request__id_seq", "plugin_idc_core_commerce_assisted_quote_request_item_id_seq"},
		{"plugin_idc_core_commerce_configuration_snapshot_item", "plugin_idc_core_commerce_configuration_snapshot__id_seq", "plugin_idc_core_commerce_configuration_snapshot_item_id_seq"},
		{"plugin_idc_core_commerce_quote_adjustment_snapshot", "plugin_idc_core_commerce_quote_adjustment_snapsh_id_seq", "plugin_idc_core_commerce_quote_adjustment_snapshot_id_seq"},
		{"plugin_idc_core_customer_custom_field_value_option", "plugin_idc_core_customer_custom_field_value_opti_id_seq", "plugin_idc_core_customer_custom_field_value_option_id_seq"},
		{"plugin_idc_core_marketing_campaign_newcomer_request", "plugin_idc_core_marketing_campaign_newcomer_requ_id_seq", "plugin_idc_core_marketing_campaign_newcomer_request_id_seq"},
		{"plugin_idc_core_server_accessory_entitlement_event", "plugin_idc_core_server_accessory_entitlement_eve_id_seq", "plugin_idc_core_server_accessory_entitlement_event_id_seq"},
		{"plugin_idc_core_server_accessory_price_policy_version", "plugin_idc_core_server_accessory_price_policy_ve_id_seq", "plugin_idc_core_server_accessory_price_policy_version_id_seq"},
		{"plugin_idc_core_server_accessory_renewal_work_item", "plugin_idc_core_server_accessory_renewal_work_it_id_seq", "plugin_idc_core_server_accessory_renewal_work_item_id_seq"},
	}
}
