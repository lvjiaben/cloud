// These tests ensure the staff diagnostic projection excludes free text and
// supplier references that are not resource-owned local task identifiers.
package productfulfillment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

type diagnosticDeniedAuthorizer struct{ ManagementAuthorizer }

func (diagnosticDeniedAuthorizer) AuthorizeDetail(context.Context, string) (Authorization, error) {
	return Authorization{}, context.Canceled
}

func TestDiagnosticRespectsDetailAuthorization(t *testing.T) {
	service := &serviceImpl{authorizer: diagnosticDeniedAuthorizer{}}
	if _, err := service.Detail(context.Background(), "pft_1"); err == nil {
		t.Fatal("unauthorized detail reached the database")
	}
}

func TestExecutionDiagnosticProjection(t *testing.T) {
	if sanitizeExecutionDiagnostic(nil) != nil {
		t.Fatal("invented task before creation")
	}
	input := &ExecutionDiagnostic{ResourceTaskNo: "task_0123456789abcdef0123456789abcdef", StatusCode: "processing", PhaseCode: "query", LastResultCode: "upstream_result_unknown", AttemptCount: 2}
	output := sanitizeExecutionDiagnostic(input)
	if output == input || *output != *input {
		t.Fatalf("projection changed valid facts: %+v", output)
	}
	input.ResourceTaskNo = "https://supplier/?token=PRIVATE"
	input.LastResultCode = "password=PRIVATE\nraw payload"
	input.PhaseCode = "PRIVATE arbitrary message"
	output = sanitizeExecutionDiagnostic(input)
	if output.ResourceTaskNo != "" || output.LastResultCode != "" || output.PhaseCode != "" || input.ResourceTaskNo == "" {
		t.Fatal("unsafe projection or mutated original")
	}
}

func TestExecutionDiagnosticPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_diagnostic_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteOutcomeIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoteOutcomeIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendOutcomeSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	_, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_product_fulfillment_task (
tenant_id int NOT NULL,intent_no text NOT NULL,external_reference text,status_code text,phase_code text,last_result_code text,attempt_count int,task_no text DEFAULT 'core_task',
PRIMARY KEY(tenant_id,intent_no));
INSERT INTO plugin_idc_core_product_fulfillment_task VALUES
(0,'pft_visible','task_0123456789abcdef0123456789abcdef','processing','query','upstream_result_unknown',2,'core_task'),
(9,'pft_visible','task_ffffffffffffffffffffffffffffffff','completed','submit','succeeded',9,'other_task');
CREATE TABLE plugin_idc_core_product_fulfillment_attempt (tenant_id int,task_no text,attempt_no int,finished_at timestamptz,safe_result_evidence jsonb);
INSERT INTO plugin_idc_core_product_fulfillment_attempt VALUES
(0,'core_task',1,CURRENT_TIMESTAMP,'{"failureCode":"upstream_request_rejected"}'),
(0,'core_task',2,CURRENT_TIMESTAMP,'{"failureCode":"idc_resource_provider_timeout"}'),
(0,'core_task',3,NULL,'{"failureCode":"idc_resource_provider_network"}'),
(9,'core_task',99,CURRENT_TIMESTAMP,'{"failureCode":"idc_resource_provider_auth_rejected"}');`)

	if err != nil {
		t.Fatal(err)
	}
	service := &serviceImpl{database: db}
	excerpt := `{"httpStatus":200,"response":{"status":400,"msg":"镜像在当前节点未下载"}}`
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_attempt SET safe_result_evidence=safe_result_evidence||jsonb_build_object('responseExcerpt',?::text) WHERE tenant_id=0 AND task_no='core_task' AND attempt_no=2`, excerpt); err != nil {
		t.Fatal(err)
	}
	got, err := service.loadExecutionDiagnostic(ctx, "pft_visible")
	if err != nil || got == nil || got.AttemptCount != 2 || got.FailureCode != "idc_resource_provider_timeout" || got.ResourceTaskNo != "task_0123456789abcdef0123456789abcdef" {
		t.Fatalf("projection or tenant scope failed: %+v %v", got, err)
	}
	if got.ResponseExcerpt != excerpt {
		t.Fatal("persisted supplier error response was discarded")
	}
	got, err = service.loadExecutionDiagnostic(ctx, "pft_absent")
	if err != nil || got != nil {
		t.Fatalf("invented absent task: %+v %v", got, err)
	}
}

// Arbitrary provider categories must never become staff-visible diagnostics.
func TestDiagnosticFailureCodeAllowlist(t *testing.T) {
	for _, code := range []string{"private_secret_canary", "token=private", ""} {
		if safeDiagnosticFailureCode(code) != "" {
			t.Fatalf("unreviewed code exposed: %q", code)
		}
	}
	if safeDiagnosticFailureCode("IDC_RESOURCE_PROVIDER_TIMEOUT") != "idc_resource_provider_timeout" {
		t.Fatal("safe owner classification lost")
	}
}
