package customeraddress

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

type addressBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s addressBizCtxStub) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type addressPrincipalStub struct{ audience *principal.Audience }

func (s addressPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type addressAuditStub struct{ calls int }

func (s *addressAuditStub) AppendTransition(context.Context, audit.TransitionInput) (*audit.Event, error) {
	s.calls++
	return &audit.Event{AuditNo: "AUD_address_test"}, nil
}
func (*addressAuditStub) AppendBatchTransition(context.Context, audit.BatchTransitionInput) (*audit.AppendResult, error) {
	return nil, errors.New("unexpected batch audit")
}
func (*addressAuditStub) RecordObservation(context.Context, audit.ObservationInput) (*audit.Event, error) {
	return nil, errors.New("unexpected observation audit")
}

func TestCustomerAddressLifecyclePostgreSQL(t *testing.T) {
	database := newAddressPostgreSQLDatabase(t)
	auditWriter := &addressAuditStub{}
	service := newAddressTestService(t, database, auditWriter, 41, "CUS_0123456789abcdef0123456789abcdef")
	ctx := context.Background()

	firstInput := SaveInput{Type: TypeBilling, Label: "Office", CountryCode: "cn", AddressLine1: "First street", Default: true, IdempotencyKey: "address-create-0123456789"}
	first, err := service.Save(ctx, firstInput)
	if err != nil || first == nil || !first.Default || first.CountryCode != "CN" {
		t.Fatalf("create address failed: item=%+v err=%v", first, err)
	}
	replay, err := service.Save(ctx, firstInput)
	if err != nil || replay.AddressNo != first.AddressNo || auditWriter.calls != 1 {
		t.Fatalf("address replay diverged: replay=%+v calls=%d err=%v", replay, auditWriter.calls, err)
	}
	changed := firstInput
	changed.Label = "Different"
	if _, err = service.Save(ctx, changed); !bizerr.Is(err, CodeIdempotencyConflict) {
		t.Fatalf("changed idempotent request should conflict, got %v", err)
	}

	second, err := service.Save(ctx, SaveInput{Type: TypeBilling, Label: "Home", CountryCode: "CN", AddressLine1: "Second street", Default: true, IdempotencyKey: "address-second-0123456789"})
	if err != nil || second == nil || !second.Default {
		t.Fatalf("create second default address failed: item=%+v err=%v", second, err)
	}
	items, err := service.List(ctx)
	if err != nil || len(items) != 2 {
		t.Fatalf("list addresses failed: items=%+v err=%v", items, err)
	}
	defaults := 0
	for _, item := range items {
		if item.Default {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("billing addresses contain %d defaults, want one", defaults)
	}

	foreign := newAddressTestService(t, database, &addressAuditStub{}, 42, "CUS_ffffffffffffffffffffffffffffffff")
	if _, err = foreign.Save(ctx, SaveInput{AddressNo: first.AddressNo, Type: first.Type, Label: first.Label, CountryCode: first.CountryCode, AddressLine1: first.AddressLine1, ExpectedVersion: first.Version + 1, IdempotencyKey: "foreign-address-0123456789"}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign address should be inaccessible, got %v", err)
	}

	archived, err := service.Archive(ctx, ArchiveInput{AddressNo: second.AddressNo, ExpectedVersion: second.Version, IdempotencyKey: "address-archive-0123456789"})
	if err != nil || archived == nil || archived.Status != StatusArchived || archived.Default {
		t.Fatalf("archive address failed: item=%+v err=%v", archived, err)
	}
}

func newAddressTestService(t *testing.T, database gdb.DB, auditWriter audit.Writer, userID int, customerNo string) Service {
	t.Helper()
	guard, err := customer.NewAccessGuard(addressBizCtxStub{current: bizctxcap.CurrentContext{UserID: userID, TenantID: 0}}, addressPrincipalStub{audience: &principal.Audience{UserID: userID, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: customerNo}})
	if err != nil {
		t.Fatalf("create address guard failed: %v", err)
	}
	service, err := New(database, guard, auditWriter, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create address service failed: %v", err)
	}
	return service
}

func newAddressPostgreSQLDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN to run address PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open address test database failed: %v", err)
	}
	var raw [8]byte
	if _, err = rand.Read(raw[:]); err != nil {
		t.Fatalf("generate address schema suffix failed: %v", err)
	}
	schema := "linapro_customer_address_" + hex.EncodeToString(raw[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+quoted); err != nil {
		t.Fatalf("create address test schema failed: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatalf("open scoped address database failed: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close(context.Background())
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+quoted+` CASCADE`)
		_ = admin.Close(context.Background())
	})
	statements := []string{
		`CREATE TABLE plugin_idc_core_customer_address (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,address_no VARCHAR(40) NOT NULL UNIQUE,customer_no VARCHAR(40) NOT NULL,address_type_code VARCHAR(24) NOT NULL,label VARCHAR(128) NOT NULL DEFAULT '',recipient_name VARCHAR(256) NOT NULL DEFAULT '',recipient_phone VARCHAR(32) NOT NULL DEFAULT '',country_code CHAR(2) NOT NULL,province VARCHAR(128) NOT NULL DEFAULT '',city VARCHAR(128) NOT NULL DEFAULT '',district VARCHAR(128) NOT NULL DEFAULT '',address_line1 VARCHAR(512) NOT NULL,address_line2 VARCHAR(512) NOT NULL DEFAULT '',postal_code VARCHAR(32) NOT NULL DEFAULT '',is_default BOOLEAN NOT NULL DEFAULT FALSE,status_code VARCHAR(16) NOT NULL,version BIGINT NOT NULL,created_by INT NOT NULL DEFAULT 0,updated_by INT NOT NULL DEFAULT 0,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL)`,
		`CREATE UNIQUE INDEX customer_address_default_test ON plugin_idc_core_customer_address(customer_no,address_type_code) WHERE status_code='active' AND is_default=TRUE`,
		`CREATE TABLE plugin_idc_core_idempotency_record (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,scope VARCHAR(128) NOT NULL,caller_scope_digest CHAR(64) NOT NULL,idempotency_key_digest CHAR(64) NOT NULL,request_fingerprint CHAR(64) NOT NULL,status_code VARCHAR(24) NOT NULL,resource_type VARCHAR(64) NOT NULL DEFAULT '',resource_no VARCHAR(64) NOT NULL DEFAULT '',response_status_code INT,response_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB,expires_at TIMESTAMPTZ NOT NULL,completed_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL,updated_at TIMESTAMPTZ NOT NULL,UNIQUE(tenant_id,scope,caller_scope_digest,idempotency_key_digest))`,
	}
	for _, statement := range statements {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("prepare address test schema failed: %v", err)
		}
	}
	return database
}
