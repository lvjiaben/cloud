package serviceaction

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestManagedServiceActionMigrationAndConstraintsPostgreSQL(t *testing.T) {
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
	schema := "idc_service_action_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteServiceActionIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+quoteServiceActionIdentifier(schema)+` CASCADE`)
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendServiceActionSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_product_fulfillment_intent(intent_no VARCHAR(40) PRIMARY KEY); CREATE TABLE plugin_idc_core_service_entitlement(entitlement_no VARCHAR(40) PRIMARY KEY); INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES('intent_1'); INSERT INTO plugin_idc_core_service_entitlement VALUES('ent_1')`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "098-managed-service-actions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO plugin_idc_core_managed_service_action_task(tenant_id,task_no,entitlement_no,customer_no,action_key,audience_code,actor_user_id,idempotency_key,input_snapshot,input_digest,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,interface_instance_no,interface_configuration_revision,source_intent_no,order_no,order_item_no,configuration_snapshot_no,external_reference,action_method,query_method,query_action_key) VALUES(0,'msa_1','ent_1','customer_1','cancel','customer',9,'retry_1','{}',repeat('a',64),'yunsu-ssl','v1','v1',repeat('b',64),repeat('c',64),'pfi_1',2,'intent_1','order_1','item_1','snapshot_1','supplier_1','service.cancel','service.sync','sync')`
	if _, err = database.Exec(ctx, insert); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, strings.Replace(insert, "'msa_1'", "'msa_2'", 1)); err == nil {
		t.Fatal("duplicate idempotency evidence was accepted")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_managed_service_action_task SET status_code='submitting',submit_attempt_count=2 WHERE task_no='msa_1'`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_managed_service_action_task SET submit_attempt_count=4 WHERE task_no='msa_1'`); err == nil {
		t.Fatal("unbounded submit retries were accepted")
	}
}

func quoteServiceActionIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendServiceActionSearchPath(dsn, schema string) string {
	if strings.HasPrefix(dsn, "pgsql:") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		return dsn + separator + "search_path=" + url.QueryEscape(schema)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
