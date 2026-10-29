package customeractivation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/statusflag"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const activationTestDSN = "LINAPRO_IDC_CORE_TEST_DSN"

type activationBizCtx struct{ userID int }

func (s *activationBizCtx) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: s.userID, TenantID: 0, DataScope: 1}
}

type activationPrincipal struct{ staffID int }

func (s *activationPrincipal) Current(ctx context.Context) (*principal.Audience, error) {
	userID := ctx.Value(activationUserKey{})
	if value, ok := userID.(int); ok && value == s.staffID {
		return &principal.Audience{UserID: value, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
	}
	return nil, nil
}

type activationUserKey struct{}

type activationAuthz struct {
	staffID int
	current *activationBizCtx
}

func (s activationAuthz) BatchGetPermissions(context.Context, []authz.PermissionKey) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}
func (s activationAuthz) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	out := map[authz.PermissionKey]bool{}
	if s.current.userID == s.staffID {
		out[PermissionCustomerCreate] = true
	}
	return out, nil
}
func (s activationAuthz) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}
func (s activationAuthz) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return false, nil
}
func (s activationAuthz) ReplaceRolePermissions(context.Context, authz.RoleID, []authz.PermissionKey) error {
	return nil
}

type activationAudit struct {
	calls []idccap.BusinessAuditTransition
}

func (s *activationAudit) AppendBusinessTransition(_ context.Context, item idccap.BusinessAuditTransition) error {
	s.calls = append(s.calls, item)
	return nil
}
func (*activationAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch")
}

type activationNotifications struct {
	requests []notificationcap.EventRequest
}

func (s *activationNotifications) Enqueue(_ context.Context, request notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	s.requests = append(s.requests, request)
	return notificationcap.EventReceipt{EventNo: "EVT_test"}, nil
}

type activationUsers struct {
	created      *usercap.CreateInput
	resolved     map[usercap.ResolveKey]*usercap.UserInfo
	resetContext *bizctxcap.CurrentContext
	resetCalls   *int
}

func (activationUsers) Current(context.Context) (*usercap.UserInfo, error) { return nil, nil }
func (activationUsers) Get(context.Context, usercap.UserID) (*usercap.UserInfo, error) {
	return nil, nil
}
func (activationUsers) BatchGet(context.Context, []usercap.UserID) (*capmodel.BatchResult[*usercap.UserInfo, usercap.UserID], error) {
	return nil, nil
}
func (s activationUsers) BatchResolve(context.Context, usercap.BatchResolveInput) (*capmodel.BatchResult[*usercap.UserInfo, usercap.ResolveKey], error) {
	return &capmodel.BatchResult[*usercap.UserInfo, usercap.ResolveKey]{Items: s.resolved}, nil
}
func (activationUsers) List(context.Context, usercap.ListInput) (*capmodel.PageResult[*usercap.UserInfo], error) {
	return nil, nil
}
func (activationUsers) EnsureVisible(context.Context, []usercap.UserID) error { return nil }
func (s activationUsers) Create(_ context.Context, input usercap.CreateInput) (usercap.UserID, error) {
	if s.created != nil {
		*s.created = input
	}
	return "99", nil
}
func (activationUsers) CreateFromExternal(context.Context, usercap.CreateFromExternalInput) (usercap.UserID, error) {
	return "", nil
}
func (activationUsers) Update(context.Context, usercap.UpdateInput) error { return nil }
func (activationUsers) Delete(context.Context, usercap.UserID) error      { return nil }
func (activationUsers) SetStatus(context.Context, usercap.UserID, statusflag.Enabled) error {
	return nil
}
func (s activationUsers) ResetPassword(ctx context.Context, _ usercap.UserID, _ string) error {
	if s.resetCalls != nil {
		*s.resetCalls++
	}
	if s.resetContext != nil {
		*s.resetContext = bizctxcap.CurrentFromContext(ctx)
	}
	return nil
}
func (activationUsers) Assignment() usercap.AssignmentService { return activationAssignments{} }

type activationAssignments struct{}

func (activationAssignments) ReplaceRoles(context.Context, usercap.UserID, []int) error { return nil }

func TestCustomerActivationLifecyclePostgreSQL(t *testing.T) {
	database := newActivationDatabase(t, true)
	current := &activationBizCtx{userID: 71}
	principals := &activationPrincipal{staffID: 71}
	audit := &activationAudit{}
	notifications := &activationNotifications{}
	service, err := New(database, current, principalByBizCtx{current: current, source: principals}, activationAuthz{staffID: 71, current: current}, activationUsers{}, activationLoginRecorder{}, audit, notifications, []byte(strings.Repeat("k", 32)), func(path string) (string, error) { return "https://customer.example.test" + path, nil })
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.Create(context.Background(), CreateInput{CustomerType: "individual", DisplayName: "Activation Customer", CountryCode: "CN", PreferredLocale: "zh-CN", Email: "person@example.test", Reason: "Customer requested a separate account invitation.", IdempotencyKey: "activation-create-000001"})
	if err != nil || created.Status != "active" || len(notifications.requests) != 1 || len(audit.calls) != 1 {
		t.Fatalf("created=%+v notifications=%d audits=%d err=%v", created, len(notifications.requests), len(audit.calls), err)
	}
	replayed, err := service.Create(context.Background(), CreateInput{CustomerType: "individual", DisplayName: "Activation Customer", CountryCode: "CN", PreferredLocale: "zh-CN", Email: "person@example.test", Reason: "Customer requested a separate account invitation.", IdempotencyKey: "activation-create-000001"})
	if err != nil || !replayed.Reused || replayed.CustomerNo != created.CustomerNo || len(notifications.requests) != 1 {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	activationURL, err := url.Parse(notifications.requests[0].Variables["activation_url"])
	if err != nil || activationURL.Query().Get("token") == "" {
		t.Fatalf("activation url=%q err=%v", notifications.requests[0].Variables["activation_url"], err)
	}
	if _, err = service.Redeem(context.Background(), activationURL.Query().Get("token")); err == nil {
		t.Fatal("staff identity unexpectedly redeemed customer invitation")
	}
	resent, err := service.Resend(context.Background(), ResendInput{CustomerNo: created.CustomerNo, Reason: "Administrator confirmed a new activation invitation.", IdempotencyKey: "activation-resend-000001"})
	if err != nil || resent.Reused || resent.InvitationNo == created.InvitationNo || len(notifications.requests) != 2 || len(audit.calls) != 2 {
		t.Fatalf("resent=%+v notifications=%d audits=%d err=%v", resent, len(notifications.requests), len(audit.calls), err)
	}
	resentReplay, err := service.Resend(context.Background(), ResendInput{CustomerNo: created.CustomerNo, Reason: "Administrator confirmed a new activation invitation.", IdempotencyKey: "activation-resend-000001"})
	if err != nil || !resentReplay.Reused || resentReplay.InvitationNo != resent.InvitationNo || len(notifications.requests) != 2 {
		t.Fatalf("resend replay=%+v err=%v", resentReplay, err)
	}
	resentURL, err := url.Parse(notifications.requests[1].Variables["activation_url"])
	if err != nil || resentURL.Query().Get("token") == "" {
		t.Fatalf("resent activation url=%q err=%v", notifications.requests[1].Variables["activation_url"], err)
	}
	current.userID = 99
	if _, err = service.Redeem(context.Background(), activationURL.Query().Get("token")); err == nil {
		t.Fatal("superseded invitation unexpectedly redeemed")
	}
	activated, err := service.Redeem(context.Background(), resentURL.Query().Get("token"))
	if err != nil || activated.Status != "active" || activated.CustomerNo != created.CustomerNo || len(audit.calls) != 3 {
		t.Fatalf("activated=%+v audits=%d err=%v", activated, len(audit.calls), err)
	}
	reused, err := service.Redeem(context.Background(), resentURL.Query().Get("token"))
	if err != nil || !reused.Reused {
		t.Fatalf("redeem replay=%+v err=%v", reused, err)
	}
	assertActivationValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_subject WHERE customer_no=?`, created.CustomerNo, "active")
	assertActivationCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_principal_audience WHERE sys_user_id=99 AND audience_code='customer'`, 1)
	assertActivationCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_identity_binding WHERE sys_user_id=99 AND customer_no=?`, 1, created.CustomerNo)
}

func TestCustomerDirectAccountCreatePostgreSQL(t *testing.T) {
	database := newActivationDatabase(t, true)
	current := &activationBizCtx{userID: 71}
	audit := &activationAudit{}
	notifications := &activationNotifications{}
	service, err := New(database, current, principalByBizCtx{current: current, source: &activationPrincipal{staffID: 71}}, activationAuthz{staffID: 71, current: current}, activationUsers{}, activationLoginRecorder{}, audit, notifications, []byte(strings.Repeat("k", 32)), func(path string) (string, error) { return "https://customer.example.test" + path, nil })
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateDirect(context.Background(), DirectCreateInput{CustomerType: "individual", DisplayName: "Direct Customer", CountryCode: "CN", PreferredLocale: "zh-CN", Email: "direct@example.test", PhoneCountryCode: "+86", Phone: "13800138000", InitialPassword: "initial-password-123", Reason: "Create an immediately active customer account.", IdempotencyKey: "direct-account-create-000001"})
	if err != nil || created.Status != "active" || created.UserID != 99 || len(notifications.requests) != 0 || len(audit.calls) != 1 {
		t.Fatalf("created=%+v notifications=%d audits=%d err=%v", created, len(notifications.requests), len(audit.calls), err)
	}
	replayed, err := service.CreateDirect(context.Background(), DirectCreateInput{CustomerType: "individual", DisplayName: "Direct Customer", CountryCode: "CN", PreferredLocale: "zh-CN", Email: "direct@example.test", PhoneCountryCode: "+86", Phone: "13800138000", InitialPassword: "initial-password-123", Reason: "Create an immediately active customer account.", IdempotencyKey: "direct-account-create-000001"})
	if err != nil || !replayed.Reused || replayed.CustomerNo != created.CustomerNo || len(audit.calls) != 1 {
		t.Fatalf("replayed=%+v audits=%d err=%v", replayed, len(audit.calls), err)
	}
	assertActivationValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_subject WHERE customer_no=?`, created.CustomerNo, "active")
	assertActivationValue(t, database, `SELECT email FROM plugin_idc_core_customer_contact WHERE customer_no=?`, created.CustomerNo, "direct@example.test")
	assertActivationValue(t, database, `SELECT phone FROM plugin_idc_core_customer_contact WHERE customer_no=?`, created.CustomerNo, "13800138000")
	assertActivationValue(t, database, `SELECT must_change_password::text FROM plugin_idc_core_customer_identity_binding WHERE customer_no=?`, created.CustomerNo, "false")
}

func TestCustomerInitialPasswordConfirmsPluginGateWithoutHostResetPostgreSQL(t *testing.T) {
	database := newActivationDatabase(t, true)
	current := &activationBizCtx{userID: 71}
	audit := &activationAudit{}
	var captured bizctxcap.CurrentContext
	resetCalls := 0
	users := activationUsers{resetContext: &captured, resetCalls: &resetCalls}
	staffPrincipal := principalByBizCtx{current: current, source: &activationPrincipal{staffID: 71}}
	service, err := New(database, current, staffPrincipal, activationAuthz{staffID: 71, current: current}, users, activationLoginRecorder{}, audit, &activationNotifications{}, []byte(strings.Repeat("k", 32)), func(path string) (string, error) { return "https://customer.example.test" + path, nil })
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateDirect(context.Background(), DirectCreateInput{CustomerType: "individual", DisplayName: "First Login Customer", CountryCode: "CN", PreferredLocale: "zh-CN", PhoneCountryCode: "+86", Phone: "13333333333", InitialPassword: "initial-password-123", Reason: "Create a customer for first login verification.", IdempotencyKey: "direct-first-login-000001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_core_customer_identity_binding SET must_change_password=TRUE WHERE customer_no=?`, created.CustomerNo); err != nil {
		t.Fatal(err)
	}
	current.userID = created.UserID
	customerService, err := New(database, current, activationCustomerPrincipal{userID: created.UserID, customerNo: created.CustomerNo}, activationAuthz{}, users, activationLoginRecorder{}, audit, &activationNotifications{}, []byte(strings.Repeat("k", 32)), func(path string) (string, error) { return "https://customer.example.test" + path, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = customerService.ChangeInitialPassword(context.Background(), "new-private-password-123"); err != nil {
		t.Fatal(err)
	}
	if resetCalls != 0 {
		t.Fatalf("initial-password confirmation must not call the host reset-password capability: %+v", captured)
	}
	assertActivationValue(t, database, `SELECT must_change_password::text FROM plugin_idc_core_customer_identity_binding WHERE customer_no=?`, created.CustomerNo, "false")
}

type activationCustomerPrincipal struct {
	userID     int
	customerNo string
}

func (s activationCustomerPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: s.userID, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: s.customerNo}, nil
}

func TestCustomerActivationBindsExactExistingIdentityPostgreSQL(t *testing.T) {
	database := newActivationDatabase(t, true)
	current := &activationBizCtx{userID: 71}
	principals := &activationPrincipal{staffID: 71}
	notifications := &activationNotifications{}
	users := activationUsers{resolved: map[usercap.ResolveKey]*usercap.UserInfo{
		usercap.ResolveKey("existing@example.test"): {ID: "88", Username: "existing-customer", Status: statusflag.Enabled(1)},
	}}
	service, err := New(database, current, principalByBizCtx{current: current, source: principals}, activationAuthz{staffID: 71, current: current}, users, activationLoginRecorder{}, &activationAudit{}, notifications, []byte(strings.Repeat("k", 32)), func(path string) (string, error) { return "https://customer.example.test" + path, nil })
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.Create(context.Background(), CreateInput{CustomerType: "individual", DisplayName: "Existing Identity Customer", CountryCode: "CN", PreferredLocale: "en-US", Email: "existing@example.test", BindExistingIdentity: true, Reason: "Bind the invitation to the selected existing identity.", IdempotencyKey: "activation-existing-000001"})
	if err != nil || !created.ExistingIdentityBound || created.TargetUserID != 88 || len(notifications.requests) != 1 {
		t.Fatalf("created=%+v notifications=%d err=%v", created, len(notifications.requests), err)
	}
	activationURL, parseErr := url.Parse(notifications.requests[0].Variables["activation_url"])
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	current.userID = 89
	if _, err = service.Redeem(context.Background(), activationURL.Query().Get("token")); err == nil {
		t.Fatal("a different identity unexpectedly redeemed a bound invitation")
	}
	current.userID = 88
	activated, err := service.Redeem(context.Background(), activationURL.Query().Get("token"))
	if err != nil || activated.CustomerNo != created.CustomerNo || activated.Status != "active" {
		t.Fatalf("activated=%+v err=%v", activated, err)
	}
}

func TestNormalizeCreateRejectsMalformedCountryAndPhone(t *testing.T) {
	base := CreateInput{CustomerType: "individual", DisplayName: "Phone Customer", CountryCode: "CN", PreferredLocale: "zh-CN", PhoneCountryCode: "+86", Phone: "13800138000", Reason: "Validate customer activation phone input.", IdempotencyKey: "activation-phone-000001"}
	for name, mutate := range map[string]func(*CreateInput){
		"country":      func(input *CreateInput) { input.CountryCode = "中1" },
		"calling code": func(input *CreateInput) { input.PhoneCountryCode = "+8a" },
		"phone":        func(input *CreateInput) { input.Phone = "13800-138000" },
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			mutate(&input)
			if _, err := normalizeCreate(input); err == nil {
				t.Fatal("malformed input unexpectedly accepted")
			}
		})
	}
}

func TestDirectLoginNameSeparatesPhoneAliasFromUsername(t *testing.T) {
	value, err := normalizeCreate(CreateInput{
		CustomerType: "individual", DisplayName: "Phone Customer", CountryCode: "CN",
		PreferredLocale: "zh-CN", PhoneCountryCode: "+86", Phone: "13333333333",
		Reason: "Create a directly usable phone login.", IdempotencyKey: "direct-phone-login-000001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := directLoginName(value); strings.Trim(got, "0123456789") == "" || strings.Contains(got, "@") || got == value.Phone || got == value.contactValue || len(got) < 2 || len(got) > 64 || got != directLoginName(value) {
		t.Fatalf("direct account username is not an independent stable identifier: %q", got)
	}
	email := value
	email.contactKind, email.contactValue = "email", "current@example.test"
	if got := directLoginName(email); strings.Contains(got, "@") || strings.Trim(got, "0123456789") == "" || got == directLoginName(value) {
		t.Fatalf("email and phone namespaces leaked into usernames: %q", got)
	}
	if value.contactValue != "+8613333333333" {
		t.Fatalf("structured contact value=%q", value.contactValue)
	}
}

func TestCustomerActivationExpiryWorkerPostgreSQL(t *testing.T) {
	database := newActivationDatabase(t, true)
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,customer_type_code,status_code,display_name,search_name,country_code,preferred_locale,version,created_by,updated_by,created_at,updated_at) VALUES (0,'CUS_11111111111111111111111111111111','individual','pending_activation','Expiry customer','expiry customer','CN','zh-CN',1,71,71,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_customer_contact (tenant_id,contact_no,customer_no,display_name,email,email_lookup_digest,phone_country_code,phone,phone_lookup_digest,preferred_locale,status_code,version,created_by,updated_by,created_at,updated_at) VALUES (0,'CTC_11111111111111111111111111111111','CUS_11111111111111111111111111111111','Expiry customer','expiry@example.test',?,'','',?,'zh-CN','active',1,71,71,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, strings.Repeat("1", 64), strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_customer_activation_invitation (tenant_id,invitation_no,customer_no,contact_no,token_digest,reason,idempotency_key_digest,request_fingerprint,status_code,expires_at,created_by,created_at,updated_at) VALUES (0,'INV_11111111111111111111111111111111','CUS_11111111111111111111111111111111','CTC_11111111111111111111111111111111',?,?,?,?,'active',CURRENT_TIMESTAMP - INTERVAL '1 second',71,CURRENT_TIMESTAMP - INTERVAL '1 day',CURRENT_TIMESTAMP - INTERVAL '1 day')`, strings.Repeat("2", 64), "Expiry worker test invitation reason.", strings.Repeat("3", 64), strings.Repeat("4", 64)); err != nil {
		t.Fatal(err)
	}
	worker, err := NewExpiryWorker(database)
	if err != nil {
		t.Fatal(err)
	}
	count, err := worker.Expire(context.Background(), 100)
	if err != nil || count != 1 {
		t.Fatalf("expired=%d err=%v", count, err)
	}
	assertActivationValue(t, database, `SELECT status_code FROM plugin_idc_core_customer_activation_invitation WHERE invitation_no=?`, "INV_11111111111111111111111111111111", "expired")
}

type principalByBizCtx struct {
	current *activationBizCtx
	source  *activationPrincipal
}

func (s principalByBizCtx) Current(context.Context) (*principal.Audience, error) {
	if s.current.userID == s.source.staffID {
		return &principal.Audience{UserID: s.current.userID, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
	}
	return nil, nil
}

func TestCustomerActivationMigrationPostgreSQL(t *testing.T) {
	database := newActivationDatabase(t, false)
	_, filename, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "047-customer-activation-invitation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ {
		if _, err = database.Exec(context.Background(), string(content)); err != nil {
			t.Fatalf("migration run %d: %v", run, err)
		}
	}
	assertActivationCount(t, database, `SELECT COUNT(1) FROM pg_indexes WHERE schemaname=current_schema() AND indexname='uk_idc_customer_activation_one_active_customer'`, 1)
}

func newActivationDatabase(t *testing.T, withInvitation bool) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(activationTestDSN))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "linapro_customer_activation_" + activationSuffix(t)
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
	statements := []string{
		`CREATE TABLE plugin_idc_core_customer_subject (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL DEFAULT 0,customer_no VARCHAR(40) UNIQUE NOT NULL,customer_type_code VARCHAR(16) NOT NULL,status_code VARCHAR(32) NOT NULL,display_name VARCHAR(256) NOT NULL,search_name VARCHAR(512) NOT NULL,country_code CHAR(2) NOT NULL,preferred_locale VARCHAR(16) NOT NULL,version BIGINT NOT NULL,created_by BIGINT NOT NULL,updated_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_individual_profile (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL,customer_no VARCHAR(40) UNIQUE NOT NULL,customer_type_code VARCHAR(16) NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_enterprise_profile (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL,customer_no VARCHAR(40) UNIQUE NOT NULL,customer_type_code VARCHAR(16) NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_contact (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL,contact_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40) NOT NULL,display_name VARCHAR(256) NOT NULL,email VARCHAR(320) NOT NULL,email_lookup_digest CHAR(64) NOT NULL,email_verified_at TIMESTAMPTZ,phone_country_code VARCHAR(8) NOT NULL,phone VARCHAR(32) NOT NULL,phone_lookup_digest CHAR(64) NOT NULL,phone_verified_at TIMESTAMPTZ,preferred_locale VARCHAR(16) NOT NULL,status_code VARCHAR(16) NOT NULL,version BIGINT NOT NULL,created_by BIGINT NOT NULL,updated_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,UNIQUE(customer_no,contact_no))`,
		`CREATE TABLE plugin_idc_core_customer_contact_role (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL,customer_no VARCHAR(40) NOT NULL,contact_no VARCHAR(40) NOT NULL,role_code VARCHAR(24) NOT NULL,status_code VARCHAR(16) NOT NULL,assigned_by BIGINT NOT NULL,assigned_at TIMESTAMPTZ NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_principal_audience (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL,sys_user_id BIGINT UNIQUE NOT NULL,audience_code VARCHAR(16) NOT NULL,status_code VARCHAR(16) NOT NULL,created_by BIGINT NOT NULL,updated_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE plugin_idc_core_customer_identity_binding (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL,binding_no VARCHAR(40) UNIQUE NOT NULL,sys_user_id BIGINT UNIQUE NOT NULL,customer_no VARCHAR(40) UNIQUE NOT NULL,source_code VARCHAR(24) NOT NULL,source_reference_no VARCHAR(64) NOT NULL,status_code VARCHAR(16) NOT NULL,activated_at TIMESTAMPTZ NOT NULL,must_change_password BOOLEAN NOT NULL DEFAULT FALSE,idempotency_key_digest CHAR(64),request_fingerprint CHAR(64),version BIGINT NOT NULL,created_by BIGINT NOT NULL,updated_by BIGINT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	if withInvitation {
		_, filename, _, _ := runtime.Caller(0)
		content, readErr := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "047-customer-activation-invitation.sql"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(context.Background(), string(content)); err != nil {
			t.Fatal(err)
		}
	}
	return database
}

func activationSuffix(t *testing.T) string {
	t.Helper()
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw[:])
}
func assertActivationValue(t *testing.T, database gdb.DB, query string, arg any, want string) {
	t.Helper()
	value, err := database.GetValue(context.Background(), query, arg)
	if err != nil || value.String() != want {
		t.Fatalf("value=%q want=%q err=%v", value.String(), want, err)
	}
}
func assertActivationCount(t *testing.T, database gdb.DB, query string, want int64, args ...any) {
	t.Helper()
	value, err := database.GetValue(context.Background(), query, args...)
	if err != nil || value.Int64() != want {
		t.Fatalf("count=%d want=%d err=%v", value.Int64(), want, err)
	}
}

type activationLoginRecorder struct{ username, phone *string }

func (s activationLoginRecorder) RecordProvisioned(_ context.Context, _ int, username, _ string, phone string) error {
	if s.username != nil {
		*s.username = username
	}
	if s.phone != nil {
		*s.phone = phone
	}
	return nil
}

func (activationLoginRecorder) DefaultRoleID(context.Context) (int, error) { return 2, nil }

func TestDirectPhoneLoginAliasMatchesHostPostgreSQL(t *testing.T) {
	for _, tc := range []struct{ name, country, callingCode, phone, want string }{
		{"China", "CN", "+86", "13800138000", "+8613800138000"},
		{"US", "US", "+1", "2025550123", "+12025550123"},
		{"too long", "US", "+1", "2025550123456789", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database := newActivationDatabase(t, true)
			current := &activationBizCtx{userID: 71}
			var (
				created                         usercap.CreateInput
				recordedUsername, recordedPhone string
			)
			service, err := New(database, current, principalByBizCtx{current: current, source: &activationPrincipal{staffID: 71}}, activationAuthz{staffID: 71, current: current}, activationUsers{created: &created}, activationLoginRecorder{username: &recordedUsername, phone: &recordedPhone}, &activationAudit{}, &activationNotifications{}, []byte(strings.Repeat("k", 32)), func(path string) (string, error) { return "https://customer.example.test" + path, nil })
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.CreateDirect(context.Background(), DirectCreateInput{CustomerType: "individual", DisplayName: "Phone customer", CountryCode: tc.country, PreferredLocale: "en-US", PhoneCountryCode: tc.callingCode, Phone: tc.phone, InitialPassword: "initial-password-123", Reason: "Create a phone login without an independent numeric username.", IdempotencyKey: "phone-login-alias-000001"})
			if tc.want == "" {
				if err == nil || created.Username != "" || recordedPhone != "" {
					t.Fatal("non-login phone created an account")
				}
				count, readErr := database.GetValue(context.Background(), `SELECT count(*) FROM plugin_idc_core_customer_identity_binding`)
				if readErr != nil || count.Int() != 0 {
					t.Fatal("invalid phone left a customer binding")
				}
				return
			}
			if err != nil || result == nil || created.Phone != tc.want || recordedPhone != tc.want || recordedUsername != created.Username || strings.Trim(created.Username, "0123456789") == "" || strings.Contains(created.Username, "@") {
				t.Fatalf("host/projection login identifiers differ: createdPhone=%q indexedPhone=%q err=%v", created.Phone, recordedPhone, err)
			}
			assertActivationValue(t, database, `SELECT phone FROM plugin_idc_core_customer_contact WHERE customer_no=?`, result.CustomerNo, tc.phone)
			assertActivationValue(t, database, `SELECT phone_country_code FROM plugin_idc_core_customer_contact WHERE customer_no=?`, result.CustomerNo, tc.callingCode)
		})
	}
}
