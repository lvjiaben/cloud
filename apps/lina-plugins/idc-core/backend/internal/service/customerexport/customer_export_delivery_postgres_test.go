package customerexport

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const exportPostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type exportBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s exportBizCtxStub) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type exportPrincipalStub struct{ audience *principal.Audience }

func (s exportPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type exportSessionStub struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
}

func (s exportSessionStub) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return s.current, nil
}

type exportStorageStub struct {
	storagecap.Service
	path    string
	content []byte
}

func (s exportStorageStub) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	if input.Path != s.path {
		return &storagecap.GetOutput{Found: false}, nil
	}
	return &storagecap.GetOutput{Found: true, Object: &storagecap.Object{Path: s.path, Size: int64(len(s.content)), Visibility: storagecap.VisibilityPrivate}, Body: io.NopCloser(bytes.NewReader(s.content))}, nil
}

func TestCustomerExportDeliveryPostgreSQL(t *testing.T) {
	database := newExportPostgreSQLDatabase(t)
	content := []byte("private customer archive")
	digest := sha256.Sum256(content)
	objectRef := "private/customer-exports/EXP_test/archive.zip"
	seedExportArtifact(t, database, objectRef, hex.EncodeToString(digest[:]), len(content))

	guard, err := customer.NewAccessGuard(
		exportBizCtxStub{current: bizctxcap.CurrentContext{UserID: 41, TenantID: 0}},
		exportPrincipalStub{audience: &principal.Audience{UserID: 41, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_0123456789abcdef0123456789abcdef"}},
	)
	if err != nil {
		t.Fatalf("create export access guard failed: %v", err)
	}
	delivery, err := NewDelivery(database, guard,
		exportSessionStub{current: &sessioncap.SessionInfo{ID: "session-export", UserID: "41"}},
		exportStorageStub{path: objectRef, content: content},
	)
	if err != nil {
		t.Fatalf("create export delivery failed: %v", err)
	}

	status, err := delivery.CurrentStatus(context.Background(), "EXP_0123456789abcdef0123456789abcdef")
	if err != nil || status == nil || status.Status != "ready" || status.ExpiresAt == nil {
		t.Fatalf("load export status failed: status=%+v err=%v", status, err)
	}
	ticket, err := delivery.IssueTicket(context.Background(), status.RequestNo)
	if err != nil || ticket == nil || len(ticket.Token) < 32 || !ticket.ExpiresAt.After(time.Now()) {
		t.Fatalf("issue export ticket failed: ticket=%+v err=%v", ticket, err)
	}
	download, err := delivery.Open(context.Background(), ticket.Token)
	if err != nil || download == nil || download.FileName != status.RequestNo+".zip" || download.MediaType != "application/zip" {
		t.Fatalf("open export download failed: download=%+v err=%v", download, err)
	}
	got, readErr := io.ReadAll(download.Body)
	_ = download.Body.Close()
	if readErr != nil || !bytes.Equal(got, content) {
		t.Fatalf("download content mismatch: got=%q err=%v", got, readErr)
	}
	if _, err = delivery.Open(context.Background(), ticket.Token); err == nil {
		t.Fatal("consumed export ticket should not be reusable")
	}
}

func TestCustomerExportDeliveryCollapsesForeignRequestAndSessionPostgreSQL(t *testing.T) {
	database := newExportPostgreSQLDatabase(t)
	content := []byte("private customer archive")
	digest := sha256.Sum256(content)
	objectRef := "private/customer-exports/EXP_test/archive.zip"
	seedExportArtifact(t, database, objectRef, hex.EncodeToString(digest[:]), len(content))

	guard, err := customer.NewAccessGuard(
		exportBizCtxStub{current: bizctxcap.CurrentContext{UserID: 42, TenantID: 0}},
		exportPrincipalStub{audience: &principal.Audience{UserID: 42, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_ffffffffffffffffffffffffffffffff"}},
	)
	if err != nil {
		t.Fatalf("create foreign export guard failed: %v", err)
	}
	delivery, err := NewDelivery(database, guard,
		exportSessionStub{current: &sessioncap.SessionInfo{ID: "foreign-session", UserID: "42"}},
		exportStorageStub{path: objectRef, content: content},
	)
	if err != nil {
		t.Fatalf("create foreign export delivery failed: %v", err)
	}
	if _, err = delivery.CurrentStatus(context.Background(), "EXP_0123456789abcdef0123456789abcdef"); err == nil {
		t.Fatal("foreign export request should be inaccessible")
	}
	if _, err = delivery.IssueTicket(context.Background(), "EXP_0123456789abcdef0123456789abcdef"); err == nil {
		t.Fatal("foreign export artifact should not issue a ticket")
	}
}

func newExportPostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(exportPostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run export delivery PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open export test database failed: %v", err)
	}
	schema := "linapro_customer_export_" + randomExportSuffix(t)
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
		t.Fatalf("create export test schema failed: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatalf("open scoped export database failed: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	statements := []string{
		`CREATE TABLE plugin_idc_core_customer_data_export_request (tenant_id INT NOT NULL DEFAULT 0,request_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40) NOT NULL,task_no VARCHAR(40) NOT NULL,requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE plugin_idc_core_async_task (tenant_id INT NOT NULL DEFAULT 0,task_no VARCHAR(40) PRIMARY KEY,status_code VARCHAR(32) NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_data_export_artifact (tenant_id INT NOT NULL DEFAULT 0,request_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40) NOT NULL,task_no VARCHAR(40) NOT NULL,object_ref VARCHAR(512) NOT NULL,content_sha256 CHAR(64) NOT NULL,size_bytes BIGINT NOT NULL,status_code VARCHAR(16) NOT NULL,expires_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_data_export_ticket (tenant_id INT NOT NULL DEFAULT 0,ticket_no VARCHAR(40) PRIMARY KEY,token_digest CHAR(64) NOT NULL,request_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40) NOT NULL,user_id INT NOT NULL,session_digest CHAR(64) NOT NULL,expires_at TIMESTAMPTZ NOT NULL,consumed_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("prepare export test schema failed: %v", err)
		}
	}
	return database
}

func seedExportArtifact(t *testing.T, database gdb.DB, objectRef, digest string, size int) {
	t.Helper()
	ctx := context.Background()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_data_export_request (request_no,customer_no,task_no) VALUES ('EXP_0123456789abcdef0123456789abcdef','CUS_0123456789abcdef0123456789abcdef','TSK_0123456789abcdef0123456789abcdef')`); err != nil {
		t.Fatalf("seed export request failed: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_async_task (task_no,status_code) VALUES ('TSK_0123456789abcdef0123456789abcdef','succeeded')`); err != nil {
		t.Fatalf("seed export task failed: %v", err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_data_export_artifact (request_no,customer_no,task_no,object_ref,content_sha256,size_bytes,status_code,expires_at) VALUES ('EXP_0123456789abcdef0123456789abcdef','CUS_0123456789abcdef0123456789abcdef','TSK_0123456789abcdef0123456789abcdef',?,?,?,'ready',CURRENT_TIMESTAMP + INTERVAL '1 hour')`, objectRef, digest, size); err != nil {
		t.Fatalf("seed export artifact failed: %v", err)
	}
}

func randomExportSuffix(t *testing.T) string {
	t.Helper()
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("generate export schema suffix failed: %v", err)
	}
	return hex.EncodeToString(raw[:])
}
