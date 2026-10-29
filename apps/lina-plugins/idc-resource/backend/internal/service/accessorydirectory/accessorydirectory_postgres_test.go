// This file verifies batched accessory publication, stable identities and transaction rollback.
package accessorydirectory

import (
	"context"
	"crypto/rand"
	"database/sql"
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
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type accessoryBatchDatabase struct {
	gdb.DB
	itemWrites int
}

func (d *accessoryBatchDatabase) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if strings.HasPrefix(query, "INSERT INTO plugin_idc_resource_accessory_snapshot_item") {
		d.itemWrites++
	}
	return d.DB.Exec(ctx, query, args...)
}

func TestAccessoryBatchPublishRetainsIdentityAndRollsBackPostgreSQL(t *testing.T) {
	database := newAccessoryTestDatabase(t)
	counted := &accessoryBatchDatabase{DB: database}
	service, err := New(counted)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	now := time.Now().UTC()
	input := PublishInput{MappingNo: "mapping", LocalResourceNo: "resource", ChannelNo: "channel", ExternalResourceRef: "server", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(15 * time.Minute)}}
	// Cross multiple batches, including a final partial batch.
	for index := range 1001 {
		input.Snapshot.Items = append(input.Snapshot.Items, resourcecap.AccessoryItem{
			Kind: resourcecap.AccessoryKindDisk, ExternalRef: fmt.Sprintf("disk-%04d", index), DisplayName: fmt.Sprintf("Disk %04d", index), State: resourcecap.AccessoryStateAttached, Data: []byte(`{"sizeGiB":10,"system":false}`),
		})
	}
	first, err := service.Publish(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1001 || counted.itemWrites != 3 {
		t.Fatalf("items=%d writes=%d", len(first.Items), counted.itemWrites)
	}
	second, err := service.Publish(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.SnapshotNo == second.SnapshotNo || len(second.Items) != len(first.Items) {
		t.Fatal("replacement snapshot was not published")
	}
	for index := range first.Items {
		if first.Items[index].ObjectNo != second.Items[index].ObjectNo {
			t.Fatal("stable object identity changed")
		}
	}
	// Failure in the second batch must also roll back the first batch and the
	// superseding of the previous complete snapshot.
	if _, err = database.Exec(ctx, `CREATE FUNCTION reject_accessory_batch() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.external_ref='disk-0750' THEN RAISE EXCEPTION 'fixture failure'; END IF; RETURN NEW; END; $$;
CREATE TRIGGER reject_accessory_batch BEFORE INSERT ON plugin_idc_resource_accessory_snapshot_item FOR EACH ROW EXECUTE FUNCTION reject_accessory_batch()`); err != nil {
		t.Fatal(err)
	}
	counted.itemWrites = 0
	if _, err = service.Publish(ctx, input); err == nil {
		t.Fatal("injected second-batch failure succeeded")
	}
	if counted.itemWrites != 2 {
		t.Fatalf("failure reached %d batches", counted.itemWrites)
	}
	retained, err := service.Read(ctx, "resource")
	if err != nil || retained.SnapshotNo != second.SnapshotNo {
		t.Fatalf("previous complete snapshot lost: %v", err)
	}
	headers, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_accessory_snapshot`)
	if err != nil || headers.Int() != 2 {
		t.Fatalf("partial header retained: %v", err)
	}
	items, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_accessory_snapshot_item`)
	if err != nil || items.Int() != 2002 {
		t.Fatalf("partial items retained: %v", err)
	}
	// An empty replacement is valid and does not emit an invalid empty INSERT.
	input.Snapshot.Items = nil
	counted.itemWrites = 0
	empty, err := service.Publish(ctx, input)
	if err != nil || len(empty.Items) != 0 || counted.itemWrites != 0 {
		t.Fatalf("empty snapshot failed: %v", err)
	}
}

func newAccessoryTestDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN")
	}
	ctx := context.WithoutCancel(t.Context())
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_accessory_batch_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_resource_channel (channel_no TEXT PRIMARY KEY);
CREATE TABLE plugin_idc_resource_mapping (tenant_id INT NOT NULL DEFAULT 0,mapping_no TEXT PRIMARY KEY,local_resource_no TEXT,channel_no TEXT,external_resource_ref TEXT,status_code TEXT);
INSERT INTO plugin_idc_resource_channel VALUES ('channel');
INSERT INTO plugin_idc_resource_mapping VALUES (0,'mapping','resource','channel','server','active')`); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture path unavailable")
	}
	ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/016-resource-accessory-directory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	return database
}
