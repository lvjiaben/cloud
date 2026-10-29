package financeops

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/xuri/excelize/v2"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

type financeOperationsAuthorizerStub struct {
	userID    int
	dataScope int
	denied    bool
}

func (s *financeOperationsAuthorizerStub) allow() (int, error) {
	if s.denied {
		return 0, errors.New("permission revoked")
	}
	return s.userID, nil
}

func (s *financeOperationsAuthorizerStub) AuthorizeFinancialOverview(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeInvoiceList(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeInvoiceDetail(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeTransactionList(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeTransactionDetail(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeAdjustmentList(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeAdjustmentDetail(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeManualReceiptCreate(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeBalanceAdjustmentCreate(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeInvoiceWaiverCreate(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeAdjustmentReview(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeAdjustmentPolicyWrite(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeInvoiceResend(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeInternalNoteList(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeInternalNoteCreate(context.Context, string) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizePersonalFilter(context.Context) (int, error) {
	return s.allow()
}
func (s *financeOperationsAuthorizerStub) AuthorizeFinancialExport(context.Context) (billing.ManagementFinanceAuthorization, error) {
	userID, err := s.allow()
	return billing.ManagementFinanceAuthorization{UserID: userID, DataScope: s.dataScope}, err
}

type financeOperationsAuditStub struct{}

func (financeOperationsAuditStub) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return errors.New("missing audit snapshots")
	}
	switch input.ResultCode {
	case "succeeded", "denied", "failed", "result_unknown":
	default:
		return errors.New("invalid audit result")
	}

	return nil
}
func (financeOperationsAuditStub) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type financeOperationsNotificationStub struct {
	requests []notificationcap.EventRequest
}

func (s *financeOperationsNotificationStub) Enqueue(_ context.Context, request notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	s.requests = append(s.requests, request)
	return notificationcap.EventReceipt{EventNo: "EVT_0123456789abcdef0123456789abcdef", Status: "pending"}, nil
}

type financeOperationsStorageStub struct {
	storagecap.Service
	objects map[string][]byte
}

func (s *financeOperationsStorageStub) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	if s.objects == nil {
		s.objects = map[string][]byte{}
	}
	if _, exists := s.objects[input.Path]; exists && !input.Overwrite {
		return nil, errors.New("object exists")
	}
	s.objects[input.Path] = append([]byte(nil), content...)
	return &storagecap.PutOutput{Object: &storagecap.Object{Path: input.Path, Size: int64(len(content)), ContentType: input.ContentType, Visibility: storagecap.VisibilityPrivate}}, nil
}
func (s *financeOperationsStorageStub) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	content, exists := s.objects[input.Path]
	if !exists {
		return &storagecap.GetOutput{Found: false}, nil
	}
	return &storagecap.GetOutput{Found: true, Object: &storagecap.Object{Path: input.Path, Size: int64(len(content)), Visibility: storagecap.VisibilityPrivate}, Body: io.NopCloser(bytes.NewReader(content))}, nil
}
func (s *financeOperationsStorageStub) Stat(_ context.Context, input storagecap.StatInput) (*storagecap.StatOutput, error) {
	content, exists := s.objects[input.Path]
	if !exists {
		return &storagecap.StatOutput{Found: false}, nil
	}
	return &storagecap.StatOutput{Found: true, Object: &storagecap.Object{Path: input.Path, Size: int64(len(content)), Visibility: storagecap.VisibilityPrivate}}, nil
}
func (s *financeOperationsStorageStub) Delete(_ context.Context, input storagecap.DeleteInput) error {
	delete(s.objects, input.Path)
	return nil
}

func TestFinanceOperationsDeliveryNotesAndPersonalFiltersPostgreSQL(t *testing.T) {
	database := newFinanceOperationsDatabase(t)
	seedFinanceOperationsData(t, database)
	authorizer := &financeOperationsAuthorizerStub{userID: 41, dataScope: 1}
	notifications := &financeOperationsNotificationStub{}
	service := newFinanceOperationsService(t, database, authorizer, notifications)
	ctx := context.Background()

	preview, err := service.PreviewInvoiceDelivery(ctx, "invoice-a")
	if err != nil || preview.MaskedEmail != "a***@e***.com" || strings.Contains(preview.MaskedEmail, "alice.billing") {
		t.Fatalf("billing destination preview was not safely masked: preview=%+v err=%v", preview, err)
	}
	result, err := service.ResendInvoice(ctx, "invoice-a", "resend-invoice-a-0001")
	if err != nil || result == nil || len(notifications.requests) != 1 {
		t.Fatalf("resend verified billing contact failed: result=%+v requests=%d err=%v", result, len(notifications.requests), err)
	}
	request := notifications.requests[0]
	if request.Recipient.CustomerNo != "customer-a" || request.Recipient.ContactNo != "contact-a" || request.Recipient.Kind != notificationcap.RecipientCustomerContact {
		t.Fatalf("resend did not use the authoritative billing contact: %+v", request.Recipient)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact (contact_no,customer_no,display_name,email,email_lookup_digest,email_verified_at,status_code) VALUES ('contact-b','customer-a','Backup','backup@example.com',repeat('b',64),CURRENT_TIMESTAMP,'active')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_contact_role (customer_no,contact_no,role_code,status_code) VALUES ('customer-a','contact-b','billing','active')`); err == nil {
		t.Fatal("a second active billing contact should be rejected")
	}

	note, err := service.AddInternalNote(ctx, AddNoteInput{TargetType: "invoice", TargetNo: "invoice-a", Content: "first private note", IdempotencyKey: "note-invoice-a-0001"})
	if err != nil || note == nil {
		t.Fatalf("append note failed: note=%+v err=%v", note, err)
	}
	correction, err := service.AddInternalNote(ctx, AddNoteInput{TargetType: "invoice", TargetNo: "invoice-a", Content: "correction", CorrectsNoteNo: note.NoteNo, IdempotencyKey: "note-invoice-a-0002"})
	if err != nil || correction == nil || correction.CorrectsNoteNo != note.NoteNo {
		t.Fatalf("append correction failed: correction=%+v err=%v", correction, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_financial_internal_note SET content='changed' WHERE note_no=?`, note.NoteNo); err == nil {
		t.Fatal("append-only note accepted UPDATE")
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_financial_internal_note WHERE note_no=?`, note.NoteNo); err == nil {
		t.Fatal("append-only note accepted DELETE")
	}
	serviceNote, err := service.AddInternalNote(ctx, AddNoteInput{TargetType: "service", TargetNo: "service-a", Content: "generic entitlement note", IdempotencyKey: "note-entitlement-a-0001"})
	if err != nil || serviceNote == nil || serviceNote.TargetNo != "service-a" {
		t.Fatalf("append generic entitlement note failed: note=%+v err=%v", serviceNote, err)
	}

	if _, err = service.SavePersonalFilter(ctx, SaveFilterInput{ViewCode: "invoice", DisplayName: "My overdue", Filter: []byte(`{"statusCode":"overdue"}`)}); err != nil {
		t.Fatal(err)
	}
	authorizer.userID = 42
	filters, err := service.ListSavedFilters(ctx, "invoice")
	if err != nil || len(filters) != 0 {
		t.Fatalf("another staff member saw a personal filter: filters=%+v err=%v", filters, err)
	}
}

func TestFinanceOperationsXLSXAndTripleAuthorizationPostgreSQL(t *testing.T) {
	database := newFinanceOperationsDatabase(t)
	seedFinanceOperationsData(t, database)
	authorizer := &financeOperationsAuthorizerStub{userID: 41, dataScope: 1}
	storage := &financeOperationsStorageStub{objects: map[string][]byte{}}
	service, err := New(database, authorizer, financeOperationsAuditStub{}, &financeOperationsNotificationStub{}, storage)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	blocked, err := service.CreateExport(ctx, ExportRequestInput{ExportType: "invoice", Filter: []byte(`{}`), IdempotencyKey: "export-blocked-0001"})
	if err != nil {
		t.Fatal(err)
	}
	authorizer.dataScope = 2
	if _, err = service.GenerateExport(ctx, blocked.RequestNo); err == nil {
		t.Fatal("generation should fail after data scope changes")
	}
	authorizer.dataScope = 1

	for index, exportType := range []string{"invoice", "transaction", "adjustment", "receivable"} {
		status, createErr := service.CreateExport(ctx, ExportRequestInput{ExportType: exportType, Filter: []byte(`{}`), IdempotencyKey: "export-" + exportType + "-000" + string(rune('1'+index))})
		if createErr != nil {
			t.Fatalf("create %s export: %v", exportType, createErr)
		}
		status, createErr = service.GenerateExport(ctx, status.RequestNo)
		if createErr != nil || status.Status != "ready" || status.ExpiresAt == nil || status.ExpiresAt.Sub(status.GeneratedAt.UTC()) > exportLifetime+time.Minute {
			rows, _ := database.GetAll(ctx, `SELECT request_no,export_type_code,status_code,permission_code,data_scope_code,requested_by,generated_at FROM plugin_idc_core_financial_export_request ORDER BY id`)
			artifacts, _ := database.GetAll(ctx, `SELECT request_no,status_code,expires_at FROM plugin_idc_core_financial_export_artifact ORDER BY id`)
			t.Fatalf("generate %s XLSX: status=%+v err=%v requests=%v artifacts=%v", exportType, status, createErr, rows, artifacts)
		}
		ticket, ticketErr := service.IssueExportTicket(ctx, status.RequestNo)
		if ticketErr != nil {
			t.Fatal(ticketErr)
		}
		if exportType == "invoice" {
			authorizer.denied = true
			if _, openErr := service.OpenExport(ctx, ticket.Token); openErr == nil {
				t.Fatal("download should fail after permission revocation")
			}
			authorizer.denied = false
		}
		download, openErr := service.OpenExport(ctx, ticket.Token)
		if openErr != nil || download.FileName != status.RequestNo+".xlsx" || strings.Contains(download.FileName, ".csv") {
			t.Fatalf("open %s XLSX: download=%+v err=%v", exportType, download, openErr)
		}
		content, readErr := io.ReadAll(download.Body)
		_ = download.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		workbook, workbookErr := excelize.OpenReader(bytes.NewReader(content))
		if workbookErr != nil {
			t.Fatalf("%s artifact is not XLSX: %v", exportType, workbookErr)
		}
		rows, rowsErr := workbook.GetRows("Financial data")
		_ = workbook.Close()
		if rowsErr != nil || len(rows) < 2 {
			t.Fatalf("%s XLSX has no exported data: rows=%d err=%v", exportType, len(rows), rowsErr)
		}
	}
}

func newFinanceOperationsService(t *testing.T, database gdb.DB, authorizer Authorizer, notifications notificationcap.EventSink) Service {
	t.Helper()
	service, err := New(database, authorizer, financeOperationsAuditStub{}, notifications, &financeOperationsStorageStub{objects: map[string][]byte{}})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func newFinanceOperationsDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(financialOperationsTestDSN))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL financial operations tests", financialOperationsTestDSN)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 8)
	if _, err = rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_finops_service_" + hex.EncodeToString(raw)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendFinanceOperationsSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	})
	if _, err = database.Exec(ctx, financeOperationsServiceFixture); err != nil {
		t.Fatalf("create service fixture: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "050-financial-operations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply financial operations migration: %v", err)
	}
	return database
}

func appendFinanceOperationsSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + url.QueryEscape(schema)
}

func seedFinanceOperationsData(t *testing.T, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(context.Background(), financeOperationsSeed); err != nil {
		t.Fatalf("seed financial operations data: %v", err)
	}
}

const financeOperationsServiceFixture = `
CREATE TABLE plugin_idc_core_customer_contact (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,contact_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40) NOT NULL,display_name VARCHAR(256) NOT NULL,email VARCHAR(320) NOT NULL DEFAULT '',email_lookup_digest CHAR(64) NOT NULL DEFAULT '',email_verified_at TIMESTAMPTZ,status_code VARCHAR(16) NOT NULL DEFAULT 'active',UNIQUE(contact_no),UNIQUE(customer_no,contact_no));
CREATE TABLE plugin_idc_core_customer_contact_role (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,contact_no VARCHAR(40) NOT NULL,role_code VARCHAR(24) NOT NULL,status_code VARCHAR(16) NOT NULL DEFAULT 'active');
CREATE TABLE plugin_idc_core_commerce_order (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,order_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL);
CREATE TABLE plugin_idc_core_billing_invoice (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,invoice_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL,order_no VARCHAR(40),currency_code CHAR(3) NOT NULL,status_code VARCHAR(24) NOT NULL,total_amount_minor BIGINT NOT NULL,paid_amount_minor BIGINT NOT NULL DEFAULT 0,waived_amount_minor BIGINT NOT NULL DEFAULT 0,credit_amount_minor BIGINT NOT NULL DEFAULT 0,payment_due_at TIMESTAMPTZ,version BIGINT NOT NULL DEFAULT 1,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_service_entitlement (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,entitlement_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL,version BIGINT NOT NULL DEFAULT 1);
CREATE TABLE plugin_idc_core_billing_ledger_account (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,account_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL,currency_code CHAR(3) NOT NULL);
CREATE TABLE plugin_idc_core_billing_ledger_transaction (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,transaction_no VARCHAR(40) NOT NULL UNIQUE,transaction_type_code VARCHAR(32) NOT NULL,currency_code CHAR(3) NOT NULL,debit_total_minor BIGINT NOT NULL,source_type_code VARCHAR(32) NOT NULL,source_reference VARCHAR(128) NOT NULL,posted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE plugin_idc_core_billing_ledger_entry (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,transaction_no VARCHAR(40) NOT NULL,account_no VARCHAR(40) NOT NULL,invoice_no VARCHAR(40));
CREATE TABLE plugin_idc_core_billing_adjustment_request (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,adjustment_no VARCHAR(40) NOT NULL UNIQUE,adjustment_type_code VARCHAR(32) NOT NULL,customer_no VARCHAR(40) NOT NULL,invoice_no VARCHAR(40),currency_code CHAR(3) NOT NULL,amount_minor BIGINT NOT NULL,status_code VARCHAR(24) NOT NULL,reason_code VARCHAR(64) NOT NULL,requested_by INT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
`

const financeOperationsSeed = `
INSERT INTO plugin_idc_core_customer_contact (contact_no,customer_no,display_name,email,email_lookup_digest,email_verified_at,status_code) VALUES ('contact-a','customer-a','Billing','alice.billing@example.com',repeat('a',64),CURRENT_TIMESTAMP,'active');
INSERT INTO plugin_idc_core_customer_contact_role (customer_no,contact_no,role_code,status_code) VALUES ('customer-a','contact-a','billing','active');
INSERT INTO plugin_idc_core_commerce_order (order_no,customer_no) VALUES ('order-a','customer-a');
INSERT INTO plugin_idc_core_billing_invoice (invoice_no,customer_no,order_no,currency_code,status_code,total_amount_minor,paid_amount_minor,waived_amount_minor,credit_amount_minor,payment_due_at,version) VALUES ('invoice-a','customer-a','order-a','CNY','overdue',10000,1000,0,0,CURRENT_TIMESTAMP,3);
INSERT INTO plugin_idc_core_service_entitlement (entitlement_no,customer_no,version) VALUES ('service-a','customer-a',2),('service-b','customer-a',4);
INSERT INTO plugin_idc_core_billing_ledger_account (account_no,customer_no,currency_code) VALUES ('account-a','customer-a','CNY');
INSERT INTO plugin_idc_core_billing_ledger_transaction (transaction_no,transaction_type_code,currency_code,debit_total_minor,source_type_code,source_reference) VALUES ('transaction-a','invoice','CNY',10000,'billing_invoice','invoice-a');
INSERT INTO plugin_idc_core_billing_ledger_entry (transaction_no,account_no,invoice_no) VALUES ('transaction-a','account-a','invoice-a');
INSERT INTO plugin_idc_core_billing_adjustment_request (adjustment_no,adjustment_type_code,customer_no,invoice_no,currency_code,amount_minor,status_code,reason_code,requested_by) VALUES ('adjustment-a','invoice_waiver','customer-a','invoice-a','CNY',100,'approved','customer_care',41);
`
