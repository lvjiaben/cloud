package invoiceprofile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const invoiceProfilePostgreSQLDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type invoiceProfileCustomerStub struct{ subject identitycap.CustomerSubject }

func (s invoiceProfileCustomerStub) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	copyValue := s.subject
	return &copyValue, nil
}

type invoiceProfileSecretStub struct {
	mu      sync.Mutex
	values  map[string][]byte
	handles map[string]secretcap.Handle
}

func newInvoiceProfileSecretStub() *invoiceProfileSecretStub {
	return &invoiceProfileSecretStub{values: map[string][]byte{}, handles: map[string]secretcap.Handle{}}
}

func (s *invoiceProfileSecretStub) Create(ctx context.Context, key string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	var value []byte
	err := produce(func(input []byte) error {
		value = append([]byte(nil), input...)
		return nil
	})
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, ok := s.values[key]; ok {
		if string(previous) != string(value) {
			return secretcap.CreateResult{}, bizerr.NewCode(CodeConflict)
		}
		return secretcap.CreateResult{Handle: s.handles[key], Metadata: secretMetadata(value), Reused: true}, nil
	}
	handle, err := secretcap.NewHandle("secret://invoice-profile/" + hex.EncodeToString([]byte(key)))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	s.values[key] = append([]byte(nil), value...)
	s.handles[key] = handle
	return secretcap.CreateResult{Handle: handle, Metadata: secretMetadata(value)}, nil
}

func secretMetadata(value []byte) secretcap.Metadata {
	tail := string(value)
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	return secretcap.Metadata{Configured: true, MaskedTail: "****" + tail, UpdatedAt: time.Now().UTC()}
}

func (s *invoiceProfileSecretStub) RecoverCreate(context.Context, string) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{}, nil
}
func (s *invoiceProfileSecretStub) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, nil
}
func (s *invoiceProfileSecretStub) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}
func (s *invoiceProfileSecretStub) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}
func (s *invoiceProfileSecretStub) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func TestInvoiceProfileCurrentCustomerLifecyclePostgreSQL(t *testing.T) {
	database, cleanup := newInvoiceProfileTestDatabase(t)
	defer cleanup()
	ctx := context.Background()
	serviceValue, err := New(database, invoiceProfileCustomerStub{subject: identitycap.CustomerSubject{
		CustomerNo: "customer-1", CustomerTypeCode: "enterprise",
		PrincipalReference: "user:101", ActorUserID: 101,
	}}, newInvoiceProfileSecretStub(), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	service := serviceValue.(*serviceImpl)

	first, err := service.CreateCurrentCustomer(ctx, CreateInput{
		TitleTypeCode: TitleTypeEnterprise, InvoiceTitle: "第一企业", TaxRegistrationNo: "TAX-001",
		RegisteredAddress: "上海", RegisteredPhone: "021-1", BankName: "测试银行",
		BankAccount: []byte("6222000012345678"), IsDefault: true,
		IdempotencyKey: "invoice-profile-create-0001",
	})
	if err != nil || first.ProfileNo == "" || first.BankAccountMasked != "****5678" || !first.IsDefault {
		t.Fatalf("create first profile: item=%+v err=%v", first, err)
	}
	second, err := service.CreateCurrentCustomer(ctx, CreateInput{
		TitleTypeCode: TitleTypeEnterprise, InvoiceTitle: "第二企业", TaxRegistrationNo: "TAX-002",
		IsDefault: true, IdempotencyKey: "invoice-profile-create-0002",
	})
	if err != nil || !second.IsDefault {
		t.Fatalf("create second profile: item=%+v err=%v", second, err)
	}
	page, err := service.ListCurrentCustomer(ctx, ListInput{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("first page: %+v err=%v", page, err)
	}
	next, err := service.ListCurrentCustomer(ctx, ListInput{Limit: 10, Cursor: page.NextCursor})
	if err != nil || len(next.Items) != 1 {
		t.Fatalf("next page: %+v err=%v", next, err)
	}

	var refreshedFirst *Profile
	for _, item := range append(page.Items, next.Items...) {
		if item.ProfileNo == first.ProfileNo {
			copyValue := item
			refreshedFirst = &copyValue
		}
	}
	if refreshedFirst == nil || refreshedFirst.IsDefault {
		t.Fatalf("previous default was not cleared: %+v", refreshedFirst)
	}
	updated, err := service.UpdateCurrentCustomer(ctx, UpdateInput{
		ProfileNo: first.ProfileNo, ExpectedVersion: refreshedFirst.Version,
		TitleTypeCode: TitleTypeEnterprise, InvoiceTitle: "第一企业（更新）", TaxRegistrationNo: "TAX-001",
		RegisteredAddress: "上海", RegisteredPhone: "021-1", BankName: "测试银行",
		BankAccountAction: BankAccountKeep, IdempotencyKey: "invoice-profile-update-0001",
	})
	if err != nil || updated.BankAccountMasked != "****5678" {
		t.Fatalf("keep bank account: item=%+v err=%v", updated, err)
	}
	archived, err := service.ArchiveCurrentCustomer(ctx, VersionInput{ProfileNo: second.ProfileNo, ExpectedVersion: second.Version})
	if err != nil || archived.StatusCode != StatusArchived || archived.IsDefault {
		t.Fatalf("archive default: item=%+v err=%v", archived, err)
	}
	page, err = service.ListCurrentCustomer(ctx, ListInput{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item.IsDefault {
			t.Fatalf("archiving the default silently selected another profile: %+v", page.Items)
		}
	}
	currentFirst := findProfile(page.Items, first.ProfileNo)
	defaulted, err := service.SetDefaultCurrentCustomer(ctx, VersionInput{ProfileNo: first.ProfileNo, ExpectedVersion: currentFirst.Version})
	if err != nil || !defaulted.IsDefault {
		t.Fatalf("set default: item=%+v err=%v", defaulted, err)
	}
	cleared, err := service.UpdateCurrentCustomer(ctx, UpdateInput{
		ProfileNo: first.ProfileNo, ExpectedVersion: defaulted.Version,
		TitleTypeCode: TitleTypeIndividual, InvoiceTitle: "个人姓名",
		BankAccountAction: BankAccountClear, IdempotencyKey: "invoice-profile-update-0002",
	})
	if err != nil || cleared.BankAccountConfigured || cleared.BankAccountMasked != "" {
		t.Fatalf("clear bank and change to individual: item=%+v err=%v", cleared, err)
	}
	if _, err = service.UpdateCurrentCustomer(ctx, UpdateInput{
		ProfileNo: first.ProfileNo, ExpectedVersion: defaulted.Version,
		TitleTypeCode: TitleTypeIndividual, InvoiceTitle: "过期更新",
		BankAccountAction: BankAccountKeep, IdempotencyKey: "invoice-profile-update-stale",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale update err=%v", err)
	}
}

func findProfile(items []Profile, profileNo string) *Profile {
	for index := range items {
		if items[index].ProfileNo == profileNo {
			return &items[index]
		}
	}
	return nil
}

func newInvoiceProfileTestDatabase(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(invoiceProfilePostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run invoice profile PostgreSQL tests", invoiceProfilePostgreSQLDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var randomValue [8]byte
	if _, err = rand.Read(randomValue[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_invoice_profile_test_" + hex.EncodeToString(randomValue[:])
	quoted := `"` + schema + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	scopedDSN := dsn + "?search_path=" + schema
	if strings.Contains(dsn, "?") {
		scopedDSN = dsn + "&search_path=" + schema
	}
	database, err := gdb.New(gdb.ConfigNode{Link: scopedDSN})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, invoiceProfileTestSchema); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_ = database.Close(ctx)
		if strings.HasPrefix(schema, "linapro_invoice_profile_test_") {
			_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
		}
		_ = admin.Close(ctx)
	}
	return database, cleanup
}

const invoiceProfileTestSchema = `
CREATE TABLE plugin_idc_core_customer_invoice_profile (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    profile_no VARCHAR(40) NOT NULL UNIQUE,
    customer_no VARCHAR(40) NOT NULL,
    title_type_code VARCHAR(16) NOT NULL,
    invoice_title VARCHAR(512) NOT NULL,
    tax_registration_no VARCHAR(128) NOT NULL DEFAULT '',
    registered_address VARCHAR(512) NOT NULL DEFAULT '',
    registered_phone VARCHAR(32) NOT NULL DEFAULT '',
    bank_name VARCHAR(256) NOT NULL DEFAULT '',
    bank_account_masked VARCHAR(128) NOT NULL DEFAULT '',
    bank_account_secret_ref VARCHAR(256) NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    status_code VARCHAR(16) NOT NULL DEFAULT 'active',
    version BIGINT NOT NULL DEFAULT 1,
    created_by INT NOT NULL DEFAULT 0,
    updated_by INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX invoice_profile_default_test
ON plugin_idc_core_customer_invoice_profile (customer_no)
WHERE status_code = 'active' AND is_default = TRUE;
INSERT INTO plugin_idc_core_customer_invoice_profile
    (profile_no, customer_no, title_type_code, invoice_title, status_code, version)
VALUES ('ivp_other000000000000000000000000000', 'customer-2', 'individual', '其他客户', 'active', 1);`
