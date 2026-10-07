package customerclosure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const closurePostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type closureBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s closureBizCtxStub) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type closurePrincipalStub struct{ audience *principal.Audience }

func (s closurePrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type closureAuthorizerStub struct{ err error }

func (s closureAuthorizerStub) AuthorizeTransition(context.Context, string) error { return s.err }

type closurePrecheckStub struct {
	findings []customer.ClosureFinding
	err      error
}

func (s closurePrecheckStub) BatchCheck(_ context.Context, customerNos []string) ([]customer.ClosurePrecheckResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []customer.ClosurePrecheckResult{{
		CustomerNo: customerNos[0], ContractVersion: 1, CheckedAt: time.Now().UTC(), Findings: s.findings,
	}}, nil
}

type closureAuditStub struct {
	fail  bool
	calls int
}

func (s *closureAuditStub) AppendTransition(context.Context, audit.TransitionInput) (*audit.Event, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("forced closure audit failure")
	}
	return &audit.Event{AuditNo: "AUD_closure_test"}, nil
}

func (*closureAuditStub) AppendBatchTransition(context.Context, audit.BatchTransitionInput) (*audit.AppendResult, error) {
	return nil, errors.New("unexpected batch audit")
}

func (*closureAuditStub) RecordObservation(context.Context, audit.ObservationInput) (*audit.Event, error) {
	return nil, errors.New("unexpected observation audit")
}

func TestCustomerClosureCompletionPostgreSQL(t *testing.T) {
	database := newClosurePostgreSQLDatabase(t)
	seedClosureCustomer(t, database)
	auditWriter := &closureAuditStub{}
	service := newClosureTestService(t, database, closurePrecheckStub{findings: []customer.ClosureFinding{{Code: customer.ClosureFindingLegalRetention, Count: 2}}}, auditWriter)

	result, err := service.Complete(context.Background(), CompleteInput{
		RequestNo: "CLR_0123456789abcdef0123456789abcdef", CustomerNo: "CUS_0123456789abcdef0123456789abcdef", IdempotencyKey: "closure-complete-0123456789",
	})
	if err != nil || result == nil || result.Status != customer.ClosureStatusCompleted || result.CompletedAt.IsZero() {
		t.Fatalf("complete customer closure failed: result=%+v err=%v", result, err)
	}
	assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_subject`, "anonymized")
	assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_principal_audience`, "revoked")
	assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_identity_binding`, "revoked")
	assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_contact`, "archived")
	assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_closure_request`, "completed")

	replay, err := service.Complete(context.Background(), CompleteInput{
		RequestNo: result.RequestNo, CustomerNo: result.CustomerNo, IdempotencyKey: "closure-complete-0123456789",
	})
	if err != nil || replay == nil || !replay.CompletedAt.Equal(result.CompletedAt) || auditWriter.calls != 1 {
		t.Fatalf("closure replay diverged: replay=%+v calls=%d err=%v", replay, auditWriter.calls, err)
	}
}

func TestCustomerClosureFinalPrecheckAndAuditRollbackPostgreSQL(t *testing.T) {
	t.Run("new blocker", func(t *testing.T) {
		database := newClosurePostgreSQLDatabase(t)
		seedClosureCustomer(t, database)
		service := newClosureTestService(t, database, closurePrecheckStub{findings: []customer.ClosureFinding{{Code: customer.ClosureFindingActiveServices, Count: 1}}}, &closureAuditStub{})
		if _, err := service.Complete(context.Background(), CompleteInput{RequestNo: "CLR_0123456789abcdef0123456789abcdef", CustomerNo: "CUS_0123456789abcdef0123456789abcdef", IdempotencyKey: "closure-blocked-0123456789"}); err == nil {
			t.Fatal("final precheck blocker should reject closure")
		}
		assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_subject`, "active")
	})

	t.Run("audit rollback", func(t *testing.T) {
		database := newClosurePostgreSQLDatabase(t)
		seedClosureCustomer(t, database)
		service := newClosureTestService(t, database, closurePrecheckStub{}, &closureAuditStub{fail: true})
		if _, err := service.Complete(context.Background(), CompleteInput{RequestNo: "CLR_0123456789abcdef0123456789abcdef", CustomerNo: "CUS_0123456789abcdef0123456789abcdef", IdempotencyKey: "closure-rollback-0123456789"}); err == nil {
			t.Fatal("audit failure should reject closure")
		}
		assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_subject`, "active")
		assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_principal_audience`, "active")
		assertClosureValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_closure_request`, "pending_review")
	})
}

func newClosureTestService(t *testing.T, database gdb.DB, precheck customer.ClosurePrecheckService, auditWriter audit.Writer) Service {
	t.Helper()
	service, err := New(database,
		closureBizCtxStub{current: bizctxcap.CurrentContext{UserID: 81, TenantID: 0}},
		closurePrincipalStub{audience: &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}},
		closureAuthorizerStub{}, precheck, auditWriter)
	if err != nil {
		t.Fatalf("create closure service failed: %v", err)
	}
	return service
}

func newClosurePostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(closurePostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run closure PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open closure test database failed: %v", err)
	}
	schema := "linapro_customer_closure_" + randomClosureSuffix(t)
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
		t.Fatalf("create closure test schema failed: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatalf("open scoped closure database failed: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	for _, statement := range closureSchemaStatements {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("prepare closure test schema failed: %v", err)
		}
	}
	return database
}

var closureSchemaStatements = []string{
	`CREATE TABLE plugin_idc_core_customer_subject (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,status_code VARCHAR(32) NOT NULL,display_name VARCHAR(256) NOT NULL,search_name VARCHAR(256) NOT NULL,country_code CHAR(2) NOT NULL,preferred_locale VARCHAR(16) NOT NULL,status_reason_code VARCHAR(64) NOT NULL DEFAULT '',status_reason VARCHAR(512) NOT NULL DEFAULT '',status_changed_by INT NOT NULL DEFAULT 0,status_changed_at TIMESTAMPTZ,anonymized_at TIMESTAMPTZ,version BIGINT NOT NULL,updated_by INT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_individual_profile (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,legal_name VARCHAR(256) NOT NULL DEFAULT '',legal_name_masked VARCHAR(256) NOT NULL DEFAULT '',identity_identifier_masked VARCHAR(128) NOT NULL DEFAULT '',identity_lookup_digest CHAR(64) NOT NULL DEFAULT '',identity_status_code VARCHAR(24) NOT NULL,identity_verified_at TIMESTAMPTZ,identity_expires_at TIMESTAMPTZ,version BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_enterprise_profile (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,legal_name VARCHAR(512) NOT NULL DEFAULT '',registration_identifier VARCHAR(128) NOT NULL DEFAULT '',registration_identifier_masked VARCHAR(128) NOT NULL DEFAULT '',registration_lookup_digest CHAR(64) NOT NULL DEFAULT '',industry_code VARCHAR(64) NOT NULL DEFAULT '',verification_status_code VARCHAR(24) NOT NULL,verified_at TIMESTAMPTZ,expires_at TIMESTAMPTZ,version BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_contact (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,display_name VARCHAR(256) NOT NULL,email VARCHAR(320) NOT NULL,email_lookup_digest CHAR(64) NOT NULL,email_verified_at TIMESTAMPTZ,phone_country_code VARCHAR(8) NOT NULL,phone VARCHAR(32) NOT NULL,phone_lookup_digest CHAR(64) NOT NULL,phone_verified_at TIMESTAMPTZ,status_code VARCHAR(16) NOT NULL,version BIGINT NOT NULL,updated_by INT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_contact_preference (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,enabled BOOLEAN NOT NULL,version BIGINT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_contact_role (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,status_code VARCHAR(16) NOT NULL,revoked_by INT NOT NULL DEFAULT 0,revoked_at TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_address (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,label VARCHAR(128) NOT NULL,recipient_name VARCHAR(256) NOT NULL,recipient_phone VARCHAR(32) NOT NULL,province VARCHAR(128) NOT NULL,city VARCHAR(128) NOT NULL,district VARCHAR(128) NOT NULL,address_line1 VARCHAR(512) NOT NULL,address_line2 VARCHAR(512) NOT NULL,postal_code VARCHAR(32) NOT NULL,is_default BOOLEAN NOT NULL,status_code VARCHAR(16) NOT NULL,version BIGINT NOT NULL,updated_by INT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_identity_binding (tenant_id INT NOT NULL DEFAULT 0,sys_user_id INT NOT NULL,customer_no VARCHAR(40) NOT NULL,status_code VARCHAR(16) NOT NULL,revoked_at TIMESTAMPTZ,version BIGINT NOT NULL,updated_by INT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_principal_audience (tenant_id INT NOT NULL DEFAULT 0,sys_user_id INT NOT NULL,audience_code VARCHAR(16) NOT NULL,status_code VARCHAR(16) NOT NULL,revoked_at TIMESTAMPTZ,version BIGINT NOT NULL,updated_by INT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
	`CREATE TABLE plugin_idc_core_customer_closure_request (tenant_id INT NOT NULL DEFAULT 0,request_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40) NOT NULL,status_code VARCHAR(24) NOT NULL,subject_version BIGINT NOT NULL,requested_at TIMESTAMPTZ NOT NULL,reviewed_by_user_id INT,reviewed_at TIMESTAMPTZ,completed_at TIMESTAMPTZ,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(request_no,customer_no))`,
}

func seedClosureCustomer(t *testing.T, database gdb.DB) {
	t.Helper()
	ctx := context.Background()
	statements := []string{
		`INSERT INTO plugin_idc_core_customer_subject (customer_no,status_code,display_name,search_name,country_code,preferred_locale,version) VALUES ('CUS_0123456789abcdef0123456789abcdef','active','Customer','customer','CN','zh-CN',1)`,
		`INSERT INTO plugin_idc_core_customer_individual_profile (customer_no,legal_name,legal_name_masked,identity_identifier_masked,identity_lookup_digest,identity_status_code,identity_verified_at,version) VALUES ('CUS_0123456789abcdef0123456789abcdef','Alice','A***','1***************2','0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef','verified',CURRENT_TIMESTAMP,1)`,
		`INSERT INTO plugin_idc_core_customer_contact (customer_no,display_name,email,email_lookup_digest,phone_country_code,phone,phone_lookup_digest,status_code,version) VALUES ('CUS_0123456789abcdef0123456789abcdef','Alice','alice@example.com','0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef','+86','13800138000','1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef','active',1)`,
		`INSERT INTO plugin_idc_core_customer_contact_preference (customer_no,enabled,version) VALUES ('CUS_0123456789abcdef0123456789abcdef',TRUE,1)`,
		`INSERT INTO plugin_idc_core_customer_contact_role (customer_no,status_code) VALUES ('CUS_0123456789abcdef0123456789abcdef','active')`,
		`INSERT INTO plugin_idc_core_customer_address (customer_no,label,recipient_name,recipient_phone,province,city,district,address_line1,address_line2,postal_code,is_default,status_code,version) VALUES ('CUS_0123456789abcdef0123456789abcdef','Home','Alice','13800138000','GD','SZ','NS','Street','','518000',TRUE,'active',1)`,
		`INSERT INTO plugin_idc_core_customer_identity_binding (sys_user_id,customer_no,status_code,version) VALUES (41,'CUS_0123456789abcdef0123456789abcdef','active',1)`,
		`INSERT INTO plugin_idc_core_principal_audience (sys_user_id,audience_code,status_code,version) VALUES (41,'customer','active',1)`,
		`INSERT INTO plugin_idc_core_customer_closure_request (request_no,customer_no,status_code,subject_version,requested_at) VALUES ('CLR_0123456789abcdef0123456789abcdef','CUS_0123456789abcdef0123456789abcdef','pending_review',1,CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatalf("seed closure fixture failed: %v", err)
		}
	}
}

func randomClosureSuffix(t *testing.T) string {
	t.Helper()
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("generate closure schema suffix failed: %v", err)
	}
	return hex.EncodeToString(raw[:])
}

func assertClosureValue(t *testing.T, database gdb.DB, query, expected string) {
	t.Helper()
	value, err := database.GetValue(context.Background(), query)
	if err != nil || value.String() != expected {
		t.Fatalf("closure value=%q want=%q err=%v", value.String(), expected, err)
	}
}
