package productfulfillment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

type outcomeReportSource struct {
	err     error
	reports []provisionresourcecap.OutcomeReport
}

func (*outcomeReportSource) CreateTask(context.Context, provisionresourcecap.TaskRequest) (*provisionresourcecap.Task, error) {
	return nil, errors.New("not used")
}

func (*outcomeReportSource) ReadTask(context.Context, provisionresourcecap.TaskReadRequest) (*provisionresourcecap.Task, error) {
	return nil, errors.New("not used")
}

func (source *outcomeReportSource) ReportOutcome(_ context.Context, report provisionresourcecap.OutcomeReport) error {
	source.reports = append(source.reports, report)
	return source.err
}

func TestResourceOutcomeWorkerDurableRetryAndLeaseRecoveryPostgreSQL(t *testing.T) {
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
	schema := "idc_resource_outcome_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteOutcomeIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+quoteOutcomeIdentifier(schema)+` CASCADE`)
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendOutcomeSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_product_fulfillment_task (task_no VARCHAR(40) PRIMARY KEY);
CREATE TABLE plugin_idc_core_product_fulfillment_resource_outcome_outbox (
 tenant_id INT NOT NULL DEFAULT 0,
 task_no VARCHAR(40) PRIMARY KEY REFERENCES plugin_idc_core_product_fulfillment_task(task_no),
 provider_task_no VARCHAR(64) NOT NULL UNIQUE,
 report_payload TEXT NOT NULL,
 report_digest CHAR(64) NOT NULL,
 status_code VARCHAR(24) NOT NULL DEFAULT 'pending',
 attempt_count INT NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 lease_token VARCHAR(40) NOT NULL DEFAULT '',
 lease_expires_at TIMESTAMPTZ,
 last_error_code VARCHAR(128) NOT NULL DEFAULT '',
 reported_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CHECK (tenant_id=0), CHECK (attempt_count BETWEEN 0 AND 10)
);`); err != nil {
		t.Fatal(err)
	}
	claimed, request := resourceWorkerFixture(productfulfillmentcap.ActionSubmit)
	resourceRequest, err := frozenResourceTaskRequest(claimed, request)
	if err != nil {
		t.Fatal(err)
	}
	report := provisionresourcecap.OutcomeReport{
		ProviderTaskNo: "resource_task_retry", Request: resourceRequest,
		Status:             provisionresourcecap.OutcomeStatusSucceeded,
		TaskEvidenceDigest: strings.Repeat("c", 64), CoreEvidenceDigest: strings.Repeat("d", 64),
	}
	payload, err := json.Marshal(report)
	if err != nil || provisionresourcecap.ValidateOutcomeReport(report) != nil {
		t.Fatalf("outcome fixture: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(task_no) VALUES('task_retry'),('task_exhausted')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_resource_outcome_outbox(task_no,provider_task_no,report_payload,report_digest) VALUES('task_retry',?,?,?)`, report.ProviderTaskNo, string(payload), sha256Hex(payload)); err != nil {
		t.Fatal(err)
	}
	failing := &outcomeReportSource{err: errors.New("temporary")}
	worker, err := NewResourceOutcomeWorker(database, failing)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := worker.Run(ctx, 1)
	if err != nil || batch.Retried != 1 || len(failing.reports) != 1 {
		t.Fatalf("first retry batch=%+v reports=%d err=%v", batch, len(failing.reports), err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_resource_outcome_outbox SET status_code='delivering',lease_token='expired_lease',lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE task_no='task_retry'`); err != nil {
		t.Fatal(err)
	}
	succeeding := &outcomeReportSource{}
	worker, _ = NewResourceOutcomeWorker(database, succeeding)
	batch, err = worker.Run(ctx, 1)
	if err != nil || batch.Reported != 1 || len(succeeding.reports) != 1 {
		t.Fatalf("lease recovery batch=%+v reports=%d err=%v", batch, len(succeeding.reports), err)
	}
	batch, err = worker.Run(ctx, 1)
	if err != nil || batch.Selected != 0 || len(succeeding.reports) != 1 {
		t.Fatalf("reported replay batch=%+v reports=%d err=%v", batch, len(succeeding.reports), err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_resource_outcome_outbox(task_no,provider_task_no,report_payload,report_digest,status_code,attempt_count,lease_token,lease_expires_at) VALUES('task_exhausted','resource_task_exhausted',?,?,'delivering',10,'expired_lease',CURRENT_TIMESTAMP-INTERVAL '1 second')`, string(payload), sha256Hex(payload)); err != nil {
		t.Fatal(err)
	}
	batch, err = worker.Run(ctx, 1)
	if err != nil || batch.Selected != 0 || len(succeeding.reports) != 1 {
		t.Fatalf("exhausted batch=%+v reports=%d err=%v", batch, len(succeeding.reports), err)
	}
	row, err := database.GetOne(ctx, `SELECT status_code,attempt_count FROM plugin_idc_core_product_fulfillment_resource_outcome_outbox WHERE task_no='task_exhausted'`)
	if err != nil || row["status_code"].String() != "manual_attention" || row["attempt_count"].Int() != 10 {
		t.Fatalf("exhausted row=%#v err=%v", row.Map(), err)
	}
}

func quoteOutcomeIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendOutcomeSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + url.QueryEscape(schema)
}
