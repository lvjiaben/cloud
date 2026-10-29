package customeroperations

import (
	"strings"
	"testing"
)

func TestNativeOperationHistoryPaginationIsLocalAndOwnedPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	seedDirectoryServer(t, f.ctx, f.db, "other-customer", "2", "active", "succeeded")
	if _, err := f.db.Exec(f.ctx, `INSERT INTO plugin_idc_resource_task(task_no,request_id_digest,capability_code,local_resource_no,channel_no,pool_no,status_code,finished_at,owner_metadata)
SELECT 'history-'||i,lpad(to_hex(i),64,'0'),'server.start',?,'channel-main','pool-main','succeeded',CURRENT_TIMESTAMP,
jsonb_build_object('operationNo','nso_'||lpad(to_hex(i),32,'0'),'action',CASE WHEN i%2=0 THEN 'reboot' ELSE 'start' END) FROM generate_series(1,25)i`, "SRV_"+strings.Repeat("1", 32)); err != nil {
		t.Fatal(err)
	}
	page, err := f.service.Operations(f.ctx, f.serverNo, 3, 10, "")
	if err != nil || page.Total != 25 || len(page.Items) != 5 || f.provider.calls != 0 || f.secrets.calls != 0 {
		t.Fatalf("local numbered history: %+v %v", page, err)
	}
	filtered, err := f.service.Operations(f.ctx, f.serverNo, 1, 10, "reboot")
	if err != nil || filtered.Total != 12 || len(filtered.Items) != 10 {
		t.Fatalf("matching total: %+v %v", filtered, err)
	}
	for _, item := range filtered.Items {
		if item.Action != "reboot" || item.AvailableOutputs == nil || len(item.AvailableOutputs) != 0 {
			t.Fatal("history leaked or mixed operations")
		}
	}
	empty, err := f.service.Operations(f.ctx, f.serverNo, 100, 10, "reboot")
	if err != nil || empty.Total != 12 || len(empty.Items) != 0 {
		t.Fatal("out-of-range page lost matching total", err)
	}
	if _, err = f.service.Operations(f.ctx, f.serverNo, 1, 51, ""); err == nil {
		t.Fatal("unbounded page accepted")
	}
	if _, err = f.service.Operations(f.ctx, f.serverNo, 1, 10, "invented_action"); err == nil {
		t.Fatal("unknown filter accepted")
	}
	f.customers.customer = "other-customer"
	if _, err = f.service.Operations(f.ctx, f.serverNo, 1, 10, ""); err == nil {
		t.Fatal("foreign history visible")
	}
}
