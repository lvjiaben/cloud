package contractautosign

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type testAuthorizer struct {
	actor    int
	err      error
	requests []managementcap.AuthorizationRequest
}

func (a *testAuthorizer) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	a.requests = append(a.requests, request)
	return a.actor, a.err
}

type testAudit struct {
	transitions []idccap.BusinessAuditTransition
}

func (a *testAudit) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	a.transitions = append(a.transitions, input)
	return nil
}

func (*testAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type testSessions struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
}

func (s *testSessions) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return s.current, nil
}

type testGateway struct{ available bool }

func (g *testGateway) TencentOnboardingStatus(context.Context) (localgatewayclient.TencentOnboardingResult, error) {
	state := "unavailable"
	if g.available {
		state = "available"
	}
	return localgatewayclient.TencentOnboardingResult{State: "active", Capabilities: []localgatewayclient.TencentOnboardingCapability{{Code: "auto_sign.enterprise", State: state}}}, nil
}

func TestEnterpriseAutoSignAuthorizationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_COMPLIANCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_COMPLIANCE_TEST_DSN to run PostgreSQL auto-sign tests")
	}
	ctx := context.Background()
	database, cleanup := autoSignTestDatabase(t, ctx, dsn)
	defer cleanup()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_template_version
  (tenant_id, template_no, version_no, name, status_code, scope_code, provider_merchant_seal_count, updated_at)
VALUES (0, 'ect_enterprise_01', 3, 'Enterprise service agreement', 'enabled', 'enterprise', 1, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert eligible template: %v", err)
	}
	audit := &testAudit{}
	gateway := &testGateway{available: true}
	sessions := &testSessions{current: &sessioncap.SessionInfo{
		ID: "session-1", UserID: "7", TenantID: capmodel.DomainID(strconv.Itoa(idccap.DefaultTenantID)), LoginAt: ptrTime(now.Add(-time.Minute)),
	}}
	service, err := New(database, managementcap.Runtime{Authorizer: &testAuthorizer{actor: 7}, Audit: audit}, sessions, gateway)
	if err != nil {
		t.Fatalf("construct auto-sign service: %v", err)
	}
	implementation := service.(*serviceImpl)
	implementation.now = func() time.Time { return now }
	input := CreateInput{
		CurrencyCode: "CNY", AllowRequiredOrder: true, MaxAmountMinor: 100_000,
		SuccessLimit: 2, ExpiresAt: now.Add(30 * 24 * time.Hour), Confirmed: true,
		IdempotencyKey: "auto-sign-create-0001", Templates: []TemplateRef{{TemplateNo: "ect_enterprise_01", VersionNo: 3}},
	}
	created, err := service.Create(ctx, input)
	if err != nil || created.StatusCode != StatusActive || len(created.Templates) != 1 {
		t.Fatalf("create authorization=%+v err=%v", created, err)
	}
	gateway.available = false
	replayed, err := service.Create(ctx, input)
	if err != nil || replayed.AuthorizationNo != created.AuthorizationNo {
		t.Fatalf("replay authorization=%+v err=%v", replayed, err)
	}
	gateway.available = true
	conflict := input
	conflict.IdempotencyKey = "auto-sign-create-0002"
	if _, err = service.Create(ctx, conflict); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("expected one-active-currency conflict, got %v", err)
	}
	invalidExpiry := input
	invalidExpiry.CurrencyCode = "EUR"
	invalidExpiry.IdempotencyKey = "auto-sign-create-expiry"
	invalidExpiry.ExpiresAt = now.AddDate(1, 0, 0).Add(time.Nanosecond)
	if _, err = service.Create(ctx, invalidExpiry); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("expected over-one-year expiry to be invalid, got %v", err)
	}
	page, err := service.List(ctx, 1)
	if err != nil || len(page.Items) != 1 || len(page.Candidates) != 1 {
		t.Fatalf("list page=%+v err=%v", page, err)
	}
	for _, size := range []int{10, 20, 50, 100} {
		first, readErr := service.List(ctx, 1, size)
		if readErr != nil || first.Total != 1 || len(first.Items) != 1 || first.PageSize != size {
			t.Fatalf("auto-sign size %d: %+v %v", size, first, readErr)
		}
		empty, readErr := service.List(ctx, 2, size)
		if readErr != nil || empty.Total != 1 || len(empty.Items) != 0 {
			t.Fatalf("auto-sign empty size %d: %+v %v", size, empty, readErr)
		}
	}
	sessions.current.LoginAt = ptrTime(now.Add(-24 * time.Hour))
	revoked, err := service.Revoke(ctx, RevokeInput{AuthorizationNo: created.AuthorizationNo, ExpectedVersion: created.Version, Confirmed: true, IdempotencyKey: "auto-sign-revoke-0001"})
	if err != nil || revoked.StatusCode != StatusRevoked {
		t.Fatalf("immediate revoke=%+v err=%v", revoked, err)
	}
	sessions.current.LoginAt = ptrTime(now.Add(-time.Minute))
	input.CurrencyCode = "USD"
	input.IdempotencyKey = "auto-sign-create-0003"
	second, err := service.Create(ctx, input)
	if err != nil {
		t.Fatalf("create second currency authorization: %v", err)
	}
	decision, err := service.Evaluate(ctx, DecisionInput{SourceCode: SourceRequired, TemplateNo: "ect_enterprise_01", VersionNo: 3, CurrencyCode: "USD", AmountMinor: 99_000})
	if err != nil || decision.AuthorizationNo != second.AuthorizationNo {
		t.Fatalf("evaluate decision=%+v err=%v", decision, err)
	}
	if _, err = service.Evaluate(ctx, DecisionInput{SourceCode: SourceOptional, TemplateNo: "ect_enterprise_01", VersionNo: 3, CurrencyCode: "USD", AmountMinor: 99_000}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("optional source should be denied, got %v", err)
	}
	if _, err = service.Evaluate(ctx, DecisionInput{SourceCode: SourceRequired, TemplateNo: "ect_enterprise_01", VersionNo: 3, CurrencyCode: "USD", AmountMinor: 100_001}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("over-limit amount should be denied, got %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_contract_template_version SET status_code = 'disabled' WHERE template_no = 'ect_enterprise_01' AND version_no = 3`); err != nil {
		t.Fatalf("disable exact template version: %v", err)
	}
	if _, err = service.Evaluate(ctx, DecisionInput{SourceCode: SourceRequired, TemplateNo: "ect_enterprise_01", VersionNo: 3, CurrencyCode: "USD", AmountMinor: 99_000}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("disabled exact template version should deny new starts, got %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_compliance_contract_template_version SET status_code = 'enabled' WHERE template_no = 'ect_enterprise_01' AND version_no = 3`); err != nil {
		t.Fatalf("restore exact template version: %v", err)
	}
	accepted := AcceptedInput{AuthorizationNo: second.AuthorizationNo, ContractNo: "contract-001", CurrencyCode: "USD", AmountMinor: 99_000, ProviderAcceptedAt: now.Add(time.Minute), IdempotencyKey: "auto-sign-accepted-0001"}
	if err = service.RecordAccepted(ctx, accepted); err != nil {
		t.Fatalf("record accepted: %v", err)
	}
	if err = service.RecordAccepted(ctx, accepted); err != nil {
		t.Fatalf("replay accepted: %v", err)
	}
	count, err := database.GetValue(ctx, `SELECT success_count FROM plugin_idc_compliance_auto_sign_auth WHERE authorization_no = ?`, second.AuthorizationNo)
	if err != nil || count.Int64() != 1 {
		t.Fatalf("accepted count=%d err=%v", count.Int64(), err)
	}
	accepted.ContractNo = "contract-002"
	if err = service.RecordAccepted(ctx, accepted); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed accepted replay should conflict, got %v", err)
	}
	accepted.IdempotencyKey = "auto-sign-accepted-0002"
	if err = service.RecordAccepted(ctx, accepted); err != nil {
		t.Fatalf("record second accepted start: %v", err)
	}
	if _, err = service.Evaluate(ctx, DecisionInput{SourceCode: SourceRequired, TemplateNo: "ect_enterprise_01", VersionNo: 3, CurrencyCode: "USD", AmountMinor: 99_000}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("used-up authorization should deny new starts, got %v", err)
	}
	deniedAuthorizer := &testAuthorizer{actor: 7, err: errors.New("denied")}
	deniedService, createErr := New(database, managementcap.Runtime{Authorizer: deniedAuthorizer, Audit: audit}, sessions, gateway)
	if createErr != nil {
		t.Fatalf("construct denied auto-sign service: %v", createErr)
	}
	if _, err = deniedService.List(ctx, 1); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("independent permission denial should hide list, got %v", err)
	}
	if len(deniedAuthorizer.requests) != 1 || deniedAuthorizer.requests[0].Permission != PermissionManage {
		t.Fatalf("authorization request=%+v want permission=%s", deniedAuthorizer.requests, PermissionManage)
	}
	if len(audit.transitions) != 3 {
		t.Fatalf("audit transition count=%d want=3", len(audit.transitions))
	}
}

func autoSignTestDatabase(t *testing.T, ctx context.Context, dsn string) (gdb.DB, func()) {
	t.Helper()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatalf("generate schema: %v", err)
	}
	schema := "idc_auto_sign_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	fixture := `
CREATE TABLE plugin_idc_compliance_contract_template_version (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id INT NOT NULL, template_no VARCHAR(64) NOT NULL, version_no BIGINT NOT NULL,
  name VARCHAR(200) NOT NULL, status_code VARCHAR(32) NOT NULL, scope_code VARCHAR(32),
  provider_merchant_seal_count INT NOT NULL DEFAULT 0, current_field_set_no VARCHAR(64),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (template_no, version_no)
);
CREATE TABLE plugin_idc_compliance_contract_template_field_set (
  tenant_id INT NOT NULL, field_set_no VARCHAR(64) PRIMARY KEY
);
CREATE TABLE plugin_idc_compliance_contract_template_field (
  tenant_id INT NOT NULL, field_set_no VARCHAR(64) NOT NULL, party_code VARCHAR(32) NOT NULL, field_type_code VARCHAR(32) NOT NULL
);
CREATE TABLE sys_dict_type (
  tenant_id INT NOT NULL, name VARCHAR(128) NOT NULL, type VARCHAR(128) NOT NULL,
  status INT NOT NULL, is_builtin INT NOT NULL, allow_tenant_override BOOLEAN NOT NULL,
  remark VARCHAR(255), created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
  tenant_id INT NOT NULL, dict_type VARCHAR(128) NOT NULL, label VARCHAR(255) NOT NULL,
  value VARCHAR(128) NOT NULL, sort INT NOT NULL, tag_style VARCHAR(32), status INT NOT NULL,
  is_builtin INT NOT NULL, created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (tenant_id, dict_type, value)
);`
	if _, err = database.Exec(ctx, fixture); err != nil {
		t.Fatalf("create template fixtures: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "014-contract-auto-sign-authorization.sql"))
	if err != nil {
		t.Fatalf("read auto-sign migration: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply auto-sign migration: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("reapply idempotent auto-sign migration: %v", err)
	}
	return database, func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
