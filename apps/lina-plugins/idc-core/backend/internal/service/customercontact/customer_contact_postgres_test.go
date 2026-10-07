package customercontact

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const contactPostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type contactTestBizCtx struct{ current bizctxcap.CurrentContext }

func (s contactTestBizCtx) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type contactTestPrincipal struct{ audience *principal.Audience }

func (s contactTestPrincipal) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type contactTestAudit struct {
	fail  bool
	calls int
}

func (s *contactTestAudit) AppendTransition(context.Context, audit.TransitionInput) (*audit.Event, error) {
	s.calls++
	if s.fail {
		return nil, errors.New("forced audit failure")
	}
	return &audit.Event{AuditNo: "AUD_contact_test"}, nil
}
func (*contactTestAudit) AppendBatchTransition(context.Context, audit.BatchTransitionInput) (*audit.AppendResult, error) {
	return nil, errors.New("unexpected batch audit")
}
func (*contactTestAudit) RecordObservation(context.Context, audit.ObservationInput) (*audit.Event, error) {
	return nil, errors.New("unexpected observation audit")
}

func TestCustomerContactLifecyclePostgreSQL(t *testing.T) {
	database := newContactPostgreSQLDatabase(t)
	auditWriter := &contactTestAudit{}
	service := newContactTestService(t, database, auditWriter, 81, "CUS_0123456789abcdef0123456789abcdef")
	ctx := context.Background()

	create := SaveInput{DisplayName: "Billing contact", Email: "billing@example.com", PhoneCountryCode: "+86", Phone: "13800138000", PreferredLocale: "zh-CN", IdempotencyKey: "contact-create-0123456789"}
	first, err := service.Save(ctx, create)
	if err != nil {
		t.Fatalf("create contact failed: %v", err)
	}
	replay, err := service.Save(ctx, create)
	if err != nil || replay.ContactNo != first.ContactNo || replay.Version != first.Version {
		t.Fatalf("contact replay diverged: first=%+v replay=%+v err=%v", first, replay, err)
	}
	changed := create
	changed.DisplayName = "Changed semantics"
	if _, err = service.Save(ctx, changed); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed idempotent request should conflict, got %v", err)
	}
	assertContactCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_contact`, 1)
	if auditWriter.calls != 1 {
		t.Fatalf("replay appended another audit event: calls=%d", auditWriter.calls)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_customer_contact SET email_verified_at=CURRENT_TIMESTAMP,phone_verified_at=CURRENT_TIMESTAMP WHERE contact_no=?`, first.ContactNo); err != nil {
		t.Fatalf("prepare verified contact failed: %v", err)
	}
	updated, err := service.Save(ctx, SaveInput{ContactNo: first.ContactNo, ExpectedVersion: first.Version, DisplayName: first.DisplayName, Email: "new@example.com", PhoneCountryCode: first.PhoneCountryCode, Phone: first.Phone, PreferredLocale: "en-US", IdempotencyKey: "contact-update-0123456789"})
	if err != nil {
		t.Fatalf("update contact failed: %v", err)
	}
	if updated.EmailVerified || !updated.PhoneVerified || updated.Version != first.Version+1 || updated.PreferredLocale != "en-US" {
		t.Fatalf("contact verification reset was incorrect: %+v", updated)
	}

	foreign := newContactTestService(t, database, &contactTestAudit{}, 82, "CUS_ffffffffffffffffffffffffffffffff")
	if _, err = foreign.Save(ctx, SaveInput{ContactNo: first.ContactNo, ExpectedVersion: updated.Version, DisplayName: updated.DisplayName, Email: updated.Email, PhoneCountryCode: updated.PhoneCountryCode, Phone: updated.Phone, PreferredLocale: updated.PreferredLocale, IdempotencyKey: "foreign-contact-0123456789"}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign customer should receive inaccessible, got %v", err)
	}

	archived, err := service.Archive(ctx, ArchiveInput{ContactNo: first.ContactNo, ExpectedVersion: updated.Version, IdempotencyKey: "contact-archive-0123456789"})
	if err != nil || archived.Status != StatusArchived || archived.Version != updated.Version+1 {
		t.Fatalf("archive contact failed: item=%+v err=%v", archived, err)
	}
	archivedReplay, err := service.Archive(ctx, ArchiveInput{ContactNo: first.ContactNo, ExpectedVersion: updated.Version, IdempotencyKey: "contact-archive-0123456789"})
	if err != nil || archivedReplay.ContactNo != archived.ContactNo || archivedReplay.Version != archived.Version {
		t.Fatalf("archive replay diverged: item=%+v err=%v", archivedReplay, err)
	}
}

func TestCustomerContactAuditFailureRollsBackPostgreSQL(t *testing.T) {
	database := newContactPostgreSQLDatabase(t)
	service := newContactTestService(t, database, &contactTestAudit{fail: true}, 81, "CUS_0123456789abcdef0123456789abcdef")
	_, err := service.Save(context.Background(), SaveInput{DisplayName: "Rollback", Email: "rollback@example.com", PreferredLocale: "zh-CN", IdempotencyKey: "contact-rollback-0123456789"})
	if err == nil {
		t.Fatal("forced audit failure should reject contact creation")
	}
	assertContactCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_customer_contact`, 0)
	assertContactCount(t, database, `SELECT COUNT(1) FROM plugin_idc_core_idempotency_record`, 0)
}

func newContactTestService(t *testing.T, database gdb.DB, auditWriter audit.Writer, userID int, customerNo string) Service {
	t.Helper()
	guard, err := customer.NewAccessGuard(contactTestBizCtx{current: bizctxcap.CurrentContext{UserID: userID, TenantID: 0}}, contactTestPrincipal{audience: &principal.Audience{UserID: userID, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: customerNo}})
	if err != nil {
		t.Fatalf("create contact access guard failed: %v", err)
	}
	service, err := New(database, guard, auditWriter, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create contact service failed: %v", err)
	}
	return service
}

func newContactPostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(contactPostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run customer contact PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open contact test database failed: %v", err)
	}
	schema := "linapro_customer_contact_test_" + randomContactTestSuffix(t)
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
		t.Fatalf("create contact test schema failed: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatalf("open scoped contact database failed: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	statements := []string{
		`CREATE TABLE plugin_idc_core_customer_contact (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,contact_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL,display_name VARCHAR(256) NOT NULL,email VARCHAR(320) NOT NULL DEFAULT '',email_lookup_digest CHAR(64) NOT NULL DEFAULT '',email_verified_at TIMESTAMPTZ,phone_country_code VARCHAR(8) NOT NULL DEFAULT '',phone VARCHAR(32) NOT NULL DEFAULT '',phone_lookup_digest CHAR(64) NOT NULL DEFAULT '',phone_verified_at TIMESTAMPTZ,preferred_locale VARCHAR(16) NOT NULL,status_code VARCHAR(16) NOT NULL,version BIGINT NOT NULL,created_by INT NOT NULL,updated_by INT NOT NULL,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,UNIQUE(customer_no,contact_no))`,
		`CREATE TABLE plugin_idc_core_customer_contact_role (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,customer_no VARCHAR(40) NOT NULL,contact_no VARCHAR(40) NOT NULL,role_code VARCHAR(24) NOT NULL,status_code VARCHAR(16) NOT NULL,assigned_by INT NOT NULL DEFAULT 0,assigned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,revoked_by INT NOT NULL DEFAULT 0,revoked_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE plugin_idc_core_idempotency_record (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,scope VARCHAR(128) NOT NULL,caller_scope_digest CHAR(64) NOT NULL,idempotency_key_digest CHAR(64) NOT NULL,request_fingerprint CHAR(64) NOT NULL,status_code VARCHAR(24) NOT NULL,lock_owner VARCHAR(128) NOT NULL DEFAULT '',lock_expires_at TIMESTAMPTZ,resource_type VARCHAR(64) NOT NULL DEFAULT '',resource_no VARCHAR(64) NOT NULL DEFAULT '',response_status_code INT,response_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB,expires_at TIMESTAMPTZ NOT NULL,completed_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,UNIQUE(tenant_id,scope,caller_scope_digest,idempotency_key_digest))`,
	}
	for _, statement := range statements {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("prepare contact test schema failed: %v", err)
		}
	}
	return database
}

func randomContactTestSuffix(t *testing.T) string {
	t.Helper()
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("generate contact test schema suffix failed: %v", err)
	}
	return hex.EncodeToString(raw[:])
}

func assertContactCount(t *testing.T, database gdb.DB, query string, expected int) {
	t.Helper()
	value, err := database.GetValue(context.Background(), query)
	if err != nil {
		t.Fatalf("count contact fixture rows failed: %v", err)
	}
	count := value.Int()
	if count != expected {
		t.Fatalf("row count=%d, want %d", count, expected)
	}
}
