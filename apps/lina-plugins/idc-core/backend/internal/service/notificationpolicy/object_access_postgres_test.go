package notificationpolicy

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

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestBusinessObjectServiceUsesGenericEntitlementPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run notification object PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "notification_object_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(t.Context(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "notification_object_") {
			_, _ = admin.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(context.Background()) })
	if _, err = database.Exec(t.Context(), `CREATE TABLE plugin_idc_core_service_entitlement (tenant_id INT NOT NULL,entitlement_no VARCHAR(40) NOT NULL,version BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_core_service_entitlement(tenant_id,entitlement_no,version) VALUES(0,'ENT_phone',7)`); err != nil {
		t.Fatal(err)
	}
	access := &businessObjectAccess{database: database}
	projection, err := access.AuthorizeCurrent(t.Context(), notificationcap.BusinessObjectRequest{Type: notificationcap.BusinessObjectService, ObjectNo: "ENT_phone"})
	if err != nil || projection.ObjectNo != "ENT_phone" || projection.CurrentVersion != 7 {
		t.Fatalf("generic entitlement projection=%+v err=%v", projection, err)
	}
	if _, err = access.AuthorizeCurrent(t.Context(), notificationcap.BusinessObjectRequest{Type: notificationcap.BusinessObjectService, ObjectNo: "SVC_missing"}); err == nil {
		t.Fatal("missing generic entitlement must fail closed")
	}
}
