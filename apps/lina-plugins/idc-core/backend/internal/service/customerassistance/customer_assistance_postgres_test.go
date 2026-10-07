package customerassistance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	authtoken "lina-core/pkg/plugin/capability/authcap/token"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const assistancePostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type assistanceBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s assistanceBizCtxStub) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type assistancePrincipalStub struct{ audience *principal.Audience }

func (s assistancePrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type assistanceAuthzStub struct {
	granted    bool
	admin      bool
	adminError error
}

func (s assistanceAuthzStub) BatchGetPermissions(context.Context, []authz.PermissionKey) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}
func (s assistanceAuthzStub) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	result := make(map[authz.PermissionKey]bool, len(keys))
	result[authz.PermissionKey(PermissionCustomerAssist)] = s.granted
	return result, nil
}
func (s assistanceAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return s.granted, nil
}
func (s assistanceAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return s.admin, s.adminError
}
func (assistanceAuthzStub) ReplaceRolePermissions(context.Context, authz.RoleID, []authz.PermissionKey) error {
	return nil
}

type assistanceAuditStub struct {
	calls []idccap.BusinessAuditTransition
	fail  bool
}

type assistanceTokenStub struct {
	issued  int
	revoked []string
}

func (*assistanceTokenStub) SelectTenant(context.Context, authtoken.SelectTenantInput) (*authtoken.TenantTokenOutput, error) {
	return nil, errors.New("unexpected tenant selection")
}
func (*assistanceTokenStub) SwitchTenant(context.Context, authtoken.SwitchTenantInput) (*authtoken.TenantTokenOutput, error) {
	return nil, errors.New("unexpected tenant switch")
}
func (s *assistanceTokenStub) IssueImpersonationToken(_ context.Context, input authtoken.ImpersonationTokenIssueInput) (*authtoken.ImpersonationTokenOutput, error) {
	s.issued++
	return &authtoken.ImpersonationTokenOutput{
		AccessToken: "host-impersonation-token-" + strings.Repeat(strconv.Itoa(s.issued), 32),
		TokenID:     "host-token-" + strconv.Itoa(s.issued), TenantID: input.TenantID,
		ActingUserID: input.ActingUserID, SubjectUserID: input.SubjectUserID,
		ExpiresAt: time.Now().Add(time.Duration(input.ExpiresInSeconds) * time.Second),
	}, nil
}
func (s *assistanceTokenStub) RevokeImpersonationToken(_ context.Context, input authtoken.ImpersonationTokenRevokeInput) error {
	s.revoked = append(s.revoked, input.BearerToken)
	return nil
}

func (s *assistanceAuditStub) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	if s.fail {
		return errors.New("forced customer assistance audit failure")
	}
	s.calls = append(s.calls, input)
	return nil
}
func (*assistanceAuditStub) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch assistance audit")
}

func TestCustomerAssistanceLifecyclePostgreSQL(t *testing.T) {
	database := newAssistancePostgreSQLDatabase(t)
	seedAssistanceCustomers(t, database)
	auditWriter := &assistanceAuditStub{}
	service := newAssistanceTestService(t, database, assistanceBizCtxStub{current: authorizedAssistanceContext(71)}, assistancePrincipalStub{audience: activeStaffAudience(71)}, assistanceAuthzStub{granted: true, adminError: errors.New("must not be queried")}, auditWriter)

	first, err := service.Start(context.Background(), StartInput{
		CustomerNo: "CUS_11111111111111111111111111111111", Reason: "Customer asked staff to verify the account summary.", TicketNo: "TKT_support_1", IdempotencyKey: "assistance-start-00000001",
	})
	if err != nil || first == nil || first.Status != "active" || len(first.AccessToken) < 32 || first.ExpiresAt.Sub(first.StartedAt) != sessionDuration {
		t.Fatalf("start assistance failed: result=%+v err=%v", first, err)
	}
	replay, err := service.Start(context.Background(), StartInput{
		CustomerNo: first.CustomerNo, Reason: "Customer asked staff to verify the account summary.", TicketNo: "TKT_support_1", IdempotencyKey: "assistance-start-00000001",
	})
	if err != nil || replay.SessionNo != first.SessionNo || replay.AccessToken != first.AccessToken || len(auditWriter.calls) != 1 {
		t.Fatalf("assistance replay diverged: replay=%+v audits=%d err=%v", replay, len(auditWriter.calls), err)
	}
	if _, err = service.Start(context.Background(), StartInput{
		CustomerNo: first.CustomerNo, Reason: "Changed reason must not reuse the previous request key.", IdempotencyKey: "assistance-start-00000001",
	}); !bizerr.Is(err, customer.CodeIdempotencyConflict) {
		t.Fatalf("changed replay should conflict, got %v", err)
	}

	snapshot, err := service.Snapshot(context.Background(), first.AccessToken)
	if err != nil || snapshot == nil || snapshot.ReadOnly || snapshot.AccessToken == "" || len(snapshot.ForbiddenActions) != 0 {
		t.Fatalf("customer assistance exchange=%+v err=%v", snapshot, err)
	}
	if snapshot.CustomerNo != first.CustomerNo {
		t.Fatalf("assistance exchange identity=%+v", snapshot)
	}
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_assistance_event WHERE session_no=? AND event_type='exchanged'`, first.SessionNo, 1)
	if _, err = service.Snapshot(context.Background(), first.AccessToken); !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("single-use assistance launch token remained usable: %v", err)
	}

	second, err := service.Start(context.Background(), StartInput{
		CustomerNo: "CUS_22222222222222222222222222222222", Reason: "Customer asked staff to check the enterprise overview.", IdempotencyKey: "assistance-start-00000002",
	})
	if err != nil || second == nil || second.SessionNo == first.SessionNo {
		t.Fatalf("second assistance start=%+v err=%v", second, err)
	}
	assertAssistanceValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_assistance_session WHERE session_no=?`, first.SessionNo, "superseded")
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_assistance_event WHERE session_no=? AND event_type='superseded'`, first.SessionNo, 1)
	if _, err = service.Snapshot(context.Background(), first.AccessToken); !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("superseded assistance token remained usable: %v", err)
	}

	ended, err := service.End(context.Background(), second.SessionNo)
	if err != nil || ended == nil || ended.Status != "ended" || ended.EndedAt == nil {
		t.Fatalf("end assistance=%+v err=%v", ended, err)
	}
	if _, err = service.Snapshot(context.Background(), second.AccessToken); !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("ended assistance token remained usable: %v", err)
	}
	if len(auditWriter.calls) != 3 || auditWriter.calls[0].OperationCode != "customer.assistance.start" || auditWriter.calls[2].OperationCode != "customer.assistance.end" {
		t.Fatalf("assistance audits=%+v", auditWriter.calls)
	}
}

func TestCustomerAssistanceExpiryAndAuditRollbackPostgreSQL(t *testing.T) {
	database := newAssistancePostgreSQLDatabase(t)
	seedAssistanceCustomers(t, database)
	service := newAssistanceTestService(t, database, assistanceBizCtxStub{current: authorizedAssistanceContext(72)}, assistancePrincipalStub{audience: activeStaffAudience(72)}, assistanceAuthzStub{granted: true}, &assistanceAuditStub{})
	item, err := service.Start(context.Background(), StartInput{CustomerNo: "CUS_11111111111111111111111111111111", Reason: "Customer requested a safe account overview check.", IdempotencyKey: "assistance-expire-000001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_core_customer_assistance_session SET expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE session_no=?`, item.SessionNo); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Snapshot(context.Background(), item.AccessToken); !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("expired assistance token returned %v", err)
	}
	if _, err = service.Snapshot(context.Background(), item.AccessToken); !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("second expired assistance read returned %v", err)
	}
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_assistance_event WHERE session_no=? AND event_type='expired'`, item.SessionNo, 1)

	backgroundItem, err := service.Start(context.Background(), StartInput{CustomerNo: "CUS_22222222222222222222222222222222", Reason: "This session must be expired by the background worker.", IdempotencyKey: "assistance-worker-000001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_core_customer_assistance_session SET expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE session_no=?`, backgroundItem.SessionNo); err != nil {
		t.Fatal(err)
	}
	worker, err := NewExpiryWorker(database)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := worker.Expire(context.Background(), 100)
	if err != nil || expired != 1 {
		t.Fatalf("background expiry count=%d err=%v", expired, err)
	}
	assertAssistanceValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_assistance_session WHERE session_no=?`, backgroundItem.SessionNo, "expired")
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_assistance_event WHERE session_no=? AND event_type='expired'`, backgroundItem.SessionNo, 1)

	failingAudit := &assistanceAuditStub{fail: true}
	failingService := newAssistanceTestService(t, database, assistanceBizCtxStub{current: authorizedAssistanceContext(73)}, assistancePrincipalStub{audience: activeStaffAudience(73)}, assistanceAuthzStub{granted: true}, failingAudit)
	if _, err = failingService.Start(context.Background(), StartInput{CustomerNo: "CUS_22222222222222222222222222222222", Reason: "This request must roll back when audit storage fails.", IdempotencyKey: "assistance-rollback-0001"}); err == nil {
		t.Fatal("audit failure did not reject assistance start")
	}
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_assistance_session WHERE staff_user_id=?`, int64(73), 0)
}

func TestCustomerAssistanceRejectsUnsafeStaffContexts(t *testing.T) {
	tests := []struct {
		name      string
		current   bizctxcap.CurrentContext
		audience  *principal.Audience
		authorize assistanceAuthzStub
	}{
		{name: "missing permission", current: authorizedAssistanceContext(80), audience: activeStaffAudience(80)},
		{name: "customer audience", current: authorizedAssistanceContext(80), audience: &principal.Audience{UserID: 80, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}, authorize: assistanceAuthzStub{granted: true}},
		{name: "impersonation", current: func() bizctxcap.CurrentContext {
			value := authorizedAssistanceContext(80)
			value.IsImpersonation = true
			value.ActingUserID = 7
			return value
		}(), audience: activeStaffAudience(80), authorize: assistanceAuthzStub{granted: true}},
		{name: "unsupported data scope", current: func() bizctxcap.CurrentContext {
			value := authorizedAssistanceContext(80)
			value.DataScopeUnsupported = true
			return value
		}(), audience: activeStaffAudience(80), authorize: assistanceAuthzStub{granted: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &serviceImpl{bizCtx: assistanceBizCtxStub{current: test.current}, principal: assistancePrincipalStub{audience: test.audience}, authorization: test.authorize}
			if _, err := service.authorize(context.Background()); !bizerr.Is(err, customer.CodeNotAccessible) {
				t.Fatalf("unsafe context returned %v", err)
			}
		})
	}
}

func TestCustomerAssistanceMigrationPostgreSQL(t *testing.T) {
	database := newAssistanceEmptyPostgreSQLDatabase(t)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve assistance test file")
	}
	migrationPath := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "046-customer-assistance-session.sql"))
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		if _, err = database.Exec(context.Background(), string(content)); err != nil {
			t.Fatalf("apply assistance migration run %d: %v", run, err)
		}
	}
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('plugin_idc_core_customer_assistance_session','plugin_idc_core_customer_assistance_event')`, nil, 2)
	assertAssistanceCount(t, database, `SELECT COUNT(1) FROM pg_indexes WHERE schemaname=current_schema() AND indexname='uk_plugin_idc_core_assistance_one_active_staff'`, nil, 1)
}

func newAssistanceTestService(t *testing.T, database gdb.DB, bizCtx assistanceBizCtxStub, principalReader assistancePrincipalStub, authorization assistanceAuthzStub, auditWriter *assistanceAuditStub) Service {
	t.Helper()
	service, err := New(database, bizCtx, principalReader, authorization, auditWriter, &assistanceTokenStub{}, []byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("create assistance service: %v", err)
	}
	return service
}

func authorizedAssistanceContext(userID int) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: userID, TenantID: 0, DataScope: 1}
}

func activeStaffAudience(userID int) *principal.Audience {
	return &principal.Audience{UserID: userID, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}
}

func newAssistancePostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(assistancePostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run customer assistance PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "linapro_customer_assistance_" + randomAssistanceSuffix(t)
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
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
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	for _, statement := range assistanceSchemaStatements {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("prepare assistance schema: %v", err)
		}
	}
	return database
}

func newAssistanceEmptyPostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(assistancePostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run customer assistance PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "linapro_customer_assistance_migration_" + randomAssistanceSuffix(t)
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
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
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	return database
}

var assistanceSchemaStatements = []string{
	`CREATE TABLE plugin_idc_core_customer_subject (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,customer_type_code VARCHAR(16) NOT NULL,status_code VARCHAR(32) NOT NULL,display_name VARCHAR(256) NOT NULL,country_code CHAR(2) NOT NULL,preferred_locale VARCHAR(16) NOT NULL)`,
	`CREATE TABLE plugin_idc_core_customer_identity_binding (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,sys_user_id BIGINT NOT NULL,status_code VARCHAR(24) NOT NULL)`,
	`CREATE TABLE plugin_idc_core_principal_audience (tenant_id INT NOT NULL DEFAULT 0,sys_user_id BIGINT NOT NULL,audience_code VARCHAR(24) NOT NULL,status_code VARCHAR(24) NOT NULL,customer_no VARCHAR(40) NOT NULL DEFAULT '')`,
	`CREATE TABLE plugin_idc_core_customer_individual_profile (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,identity_status_code VARCHAR(24) NOT NULL)`,
	`CREATE TABLE plugin_idc_core_customer_enterprise_profile (tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,verification_status_code VARCHAR(24) NOT NULL)`,
	`CREATE TABLE plugin_idc_core_customer_assistance_session (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL DEFAULT 0,session_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL,staff_user_id BIGINT NOT NULL,reason VARCHAR(512) NOT NULL,ticket_no VARCHAR(64) NOT NULL DEFAULT '',token_digest CHAR(64) NOT NULL UNIQUE,host_token_id VARCHAR(128) NOT NULL,host_token_ciphertext TEXT NOT NULL,idempotency_key_digest CHAR(64) NOT NULL,request_fingerprint CHAR(64) NOT NULL,status_code VARCHAR(16) NOT NULL,started_at TIMESTAMPTZ NOT NULL,expires_at TIMESTAMPTZ NOT NULL,exchanged_at TIMESTAMPTZ,ended_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,UNIQUE(tenant_id,staff_user_id,idempotency_key_digest))`,
	`CREATE TABLE plugin_idc_core_customer_assistance_event (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL DEFAULT 0,event_no VARCHAR(40) NOT NULL UNIQUE,session_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40) NOT NULL,staff_user_id BIGINT NOT NULL,event_type VARCHAR(16) NOT NULL,occurred_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL)`,
}

func seedAssistanceCustomers(t *testing.T, database gdb.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO plugin_idc_core_customer_subject (customer_no,customer_type_code,status_code,display_name,country_code,preferred_locale) VALUES ('CUS_11111111111111111111111111111111','individual','active','Safe Individual','CN','zh-CN')`,
		`INSERT INTO plugin_idc_core_customer_individual_profile (customer_no,identity_status_code) VALUES ('CUS_11111111111111111111111111111111','verified')`,
		`INSERT INTO plugin_idc_core_customer_identity_binding (customer_no,sys_user_id,status_code) VALUES ('CUS_11111111111111111111111111111111',1001,'active')`,
		`INSERT INTO plugin_idc_core_principal_audience (sys_user_id,audience_code,status_code,customer_no) VALUES (1001,'customer','active','CUS_11111111111111111111111111111111')`,
		`INSERT INTO plugin_idc_core_customer_subject (customer_no,customer_type_code,status_code,display_name,country_code,preferred_locale) VALUES ('CUS_22222222222222222222222222222222','enterprise','active','Safe Enterprise','US','en-US')`,
		`INSERT INTO plugin_idc_core_customer_enterprise_profile (customer_no,verification_status_code) VALUES ('CUS_22222222222222222222222222222222','pending')`,
		`INSERT INTO plugin_idc_core_customer_identity_binding (customer_no,sys_user_id,status_code) VALUES ('CUS_22222222222222222222222222222222',1002,'active')`,
		`INSERT INTO plugin_idc_core_principal_audience (sys_user_id,audience_code,status_code,customer_no) VALUES (1002,'customer','active','CUS_22222222222222222222222222222222')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
}

func assertAssistanceValue(t *testing.T, database gdb.DB, query string, argument any, expected string) {
	t.Helper()
	value, err := database.GetValue(context.Background(), query, argument)
	if err != nil || value.String() != expected {
		t.Fatalf("assistance value=%q want=%q err=%v", value.String(), expected, err)
	}
}

func assertAssistanceCount(t *testing.T, database gdb.DB, query string, argument any, expected int64) {
	t.Helper()
	var value any
	var err error
	if argument == nil {
		value, err = database.GetValue(context.Background(), query)
	} else {
		value, err = database.GetValue(context.Background(), query, argument)
	}
	variant, ok := value.(interface{ Int64() int64 })
	if !ok {
		t.Fatalf("assistance count returned unexpected value %T", value)
	}
	if err != nil || variant.Int64() != expected {
		t.Fatalf("assistance count=%d want=%d err=%v", variant.Int64(), expected, err)
	}
}

func randomAssistanceSuffix(t *testing.T) string {
	t.Helper()
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw[:])
}
