// This file proves reservation, concurrency, idempotency, and immutability against PostgreSQL.

package taxinvoicerequest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const taxInvoiceRequestPostgresDSNEnvironment = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type testBillingRow struct {
	InvoiceNo string `orm:"invoice_no"`
	OrderNo   string `orm:"order_no"`
	ItemNo    string `orm:"item_no"`
	Gross     int64  `orm:"gross_minor"`
	Net       int64  `orm:"net_minor"`
	Tax       int64  `orm:"tax_minor"`
}

type postgresSource struct {
	database   gdb.DB
	customerNo string
}

type taxInvoiceCustomer struct {
	customerNo string
}

func (c *taxInvoiceCustomer) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	return &identitycap.CustomerSubject{CustomerNo: c.customerNo}, nil
}

func (s *postgresSource) GetPostedRefund(context.Context, string) (*taxinvoicecap.RefundOperation, error) {
	return nil, errors.New("refund source is outside the request test")
}

// ListCurrentCustomerOptions returns only the current test customer; creation
// still performs the authoritative locked calculation below.
func (s *postgresSource) ListCurrentCustomerOptions(
	ctx context.Context,
	reader taxinvoicecap.AllocationUsageReader,
) (*taxinvoicecap.RequestOptions, error) {
	if _, err := reader.ReadAllocatedUsage(ctx, s.customerNo, []string{"bill_a", "bill_b"}); err != nil {
		return nil, err
	}
	return &taxinvoicecap.RequestOptions{
		CustomerNo: s.customerNo,
		Profiles:   []taxinvoicecap.ProfileSummary{{ProfileNo: "profile_1", Version: 1}},
		ObservedAt: time.Now().UTC(),
	}, nil
}

// PrepareCurrentCustomerRequest locks all selected source rows in stable order
// and calculates remaining capacity from the compliance reader in the same transaction.
func (s *postgresSource) PrepareCurrentCustomerRequest(
	ctx context.Context,
	reader taxinvoicecap.AllocationUsageReader,
	profileNo string,
	selections []taxinvoicecap.Selection,
) (result *taxinvoicecap.RequestSnapshot, err error) {
	if profileNo != "profile_1" || len(selections) == 0 {
		return nil, errors.New("invalid source request")
	}
	requested := make(map[string]int64, len(selections))
	numbers := make([]string, 0, len(selections))
	for _, selection := range selections {
		requested[selection.BillingInvoiceNo] = selection.RequestedGrossMinor
		numbers = append(numbers, selection.BillingInvoiceNo)
	}
	sort.Strings(numbers)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []testBillingRow
		if scanErr := s.database.Model("test_tax_invoice_source").Ctx(ctx).
			WhereIn("invoice_no", numbers).OrderAsc("invoice_no").LockUpdate().Scan(&rows); scanErr != nil {
			return scanErr
		}
		if len(rows) != len(numbers) {
			return errors.New("source invoice missing")
		}
		usageRows, readErr := reader.ReadAllocatedUsage(ctx, s.customerNo, numbers)
		if readErr != nil {
			return readErr
		}
		used := make(map[string]int64, len(usageRows))
		for _, usage := range usageRows {
			used[usage.BillingInvoiceNo] = usage.AllocatedGrossMinor
		}
		nowValue, nowErr := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
		if nowErr != nil {
			return nowErr
		}
		result = &taxinvoicecap.RequestSnapshot{
			CustomerNo: s.customerNo, PrincipalReference: "principal_1", ActorUserID: 1,
			Buyer: taxinvoicecap.ProfileSnapshot{ProfileSummary: taxinvoicecap.ProfileSummary{
				ProfileNo: "profile_1", TitleTypeCode: "enterprise", InvoiceTitle: "Buyer Ltd",
				TaxRegistrationNo: "buyer-tax-no", Version: 1,
			}},
			Seller: taxinvoicecap.MerchantSnapshot{
				MerchantNo: "merchant_1", LegalName: "Merchant Ltd", CountryCode: "CN", Version: 1,
			},
			CurrencyCode: "CNY", ObservedAt: nowValue.Time().UTC(),
		}
		for _, row := range rows {
			amount := requested[row.InvoiceNo]
			if amount <= 0 || amount > row.Gross-used[row.InvoiceNo] || amount%10 != 0 {
				return errors.New("requested amount exceeds remaining source capacity")
			}
			net := amount * 9 / 10
			tax := amount - net
			result.GrossAmountMinor += amount
			result.NetAmountMinor += net
			result.TaxAmountMinor += tax
			result.Invoices = append(result.Invoices, taxinvoicecap.InvoiceSnapshot{
				InvoiceOption: taxinvoicecap.InvoiceOption{
					BillingInvoiceNo: row.InvoiceNo, OrderNo: row.OrderNo, CurrencyCode: "CNY",
					SourceGrossMinor: row.Gross, AlreadyAllocatedMinor: used[row.InvoiceNo],
					RemainingGrossMinor: row.Gross - used[row.InvoiceNo], BillingInvoiceVersion: 1,
					BillingContentDigest: digestText("test-source\x00", row.InvoiceNo),
				},
				RequestedGrossMinor: amount, NetAmountMinor: net, TaxAmountMinor: tax,
				Items: []taxinvoicecap.ItemSnapshot{{
					BillingInvoiceItemNo: row.ItemNo, SourceReference: row.ItemNo,
					DescriptionZhCN: "服务器", DescriptionEnUS: "Server",
					TaxTreatmentCode: "taxable", TaxRatePPM: 111111,
					TaxClassificationCode: "server_service", GrossAmountMinor: amount,
					NetAmountMinor: net, TaxAmountMinor: tax,
					SourceContentDigest: digestText("test-item\x00", row.ItemNo),
				}},
			})
		}
		return nil
	})
	return result, err
}

type memorySecretStore struct {
	mu      sync.Mutex
	byKey   map[string]secretcap.Handle
	payload map[string][]byte
	updated time.Time
}

func newMemorySecretStore() *memorySecretStore {
	return &memorySecretStore{
		byKey: make(map[string]secretcap.Handle), payload: make(map[string][]byte),
		updated: time.Now().UTC(),
	}
}

func (s *memorySecretStore) Create(
	_ context.Context,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if handle, ok := s.byKey[key]; ok {
		return secretcap.CreateResult{
			Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: s.updated}, Reused: true,
		}, nil
	}
	var payload []byte
	if err := produce(func(value []byte) error {
		payload = append([]byte(nil), value...)
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	digest := sha256.Sum256([]byte(key))
	handle, err := secretcap.NewHandle("secret://tax-invoice/" + hex.EncodeToString(digest[:16]))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	s.byKey[key] = handle
	s.payload[handle.OpaqueRef()] = payload
	return secretcap.CreateResult{
		Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: s.updated},
	}, nil
}

func (s *memorySecretStore) RecoverCreate(_ context.Context, key string) (secretcap.CreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	handle, ok := s.byKey[key]
	if !ok {
		return secretcap.CreateResult{}, errors.New("secret not found")
	}
	return secretcap.CreateResult{
		Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: s.updated}, Reused: true,
	}, nil
}

func (*memorySecretStore) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("replace is not supported in this test")
}

func (s *memorySecretStore) Metadata(_ context.Context, handle secretcap.Handle) (secretcap.Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.payload[handle.OpaqueRef()]; !ok {
		return secretcap.Metadata{}, errors.New("secret not found")
	}
	return secretcap.Metadata{Configured: true, UpdatedAt: s.updated}, nil
}

func (*memorySecretStore) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, errors.New("revoke is not supported in this test")
}

func (*memorySecretStore) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, errors.New("rotate is not supported in this test")
}

func (s *memorySecretStore) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	s.mu.Lock()
	payload, ok := s.payload[request.Handle.OpaqueRef()]
	copyPayload := append([]byte(nil), payload...)
	s.mu.Unlock()
	if !ok {
		return errors.New("secret not found")
	}
	defer clear(copyPayload)
	return consume(copyPayload)
}

// TestCreateCurrentCustomerPostgreSQL proves combined requests, exact replay,
// changed-body conflict, reservation statuses, and immutable database guards.
func TestCreateCurrentCustomerPostgreSQL(t *testing.T) {
	database := newTaxInvoiceRequestPostgresDatabase(t)
	store := newMemorySecretStore()
	serviceValue, err := New(database, &postgresSource{database: database, customerNo: "customer_1"}, store, store, &taxInvoiceCustomer{customerNo: "customer_1"})
	if err != nil {
		t.Fatalf("construct request service failed: %v", err)
	}
	service := serviceValue.(*serviceImpl)
	input := CreateInput{
		ProfileNo: "profile_1", IdempotencyKey: "invoice-request-000001",
		Selections: []taxinvoicecap.Selection{
			{BillingInvoiceNo: "bill_a", RequestedGrossMinor: 400},
			{BillingInvoiceNo: "bill_b", RequestedGrossMinor: 300},
		},
	}
	created, err := service.CreateCurrentCustomer(context.Background(), input)
	if err != nil {
		t.Fatalf("create combined request failed: %v", err)
	}
	if created.Reused || created.StatusCode != string(StatusPendingReview) ||
		created.InvoiceTypeCode != string(InvoiceTypePendingConfirmation) ||
		created.GrossAmountMinor != 700 {
		t.Fatalf("unexpected created request: %#v", created)
	}
	replayed, err := service.CreateCurrentCustomer(context.Background(), input)
	if err != nil || !replayed.Reused || replayed.TaxInvoiceNo != created.TaxInvoiceNo {
		t.Fatalf("exact replay failed: result=%#v err=%v", replayed, err)
	}
	changed := input
	changed.Selections = append([]taxinvoicecap.Selection(nil), input.Selections...)
	changed.Selections[0].RequestedGrossMinor = 410
	if _, err = service.CreateCurrentCustomer(context.Background(), changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed-body replay error=%v want conflict", err)
	}
	usage := requireTaxInvoiceUsage(t, service, "bill_a", "bill_b")
	if usage["bill_a"] != 400 || usage["bill_b"] != 300 {
		t.Fatalf("combined usage=%v", usage)
	}
	for _, status := range []string{"failed", "result_unknown"} {
		if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_compliance_tax_invoice
SET status_code = ? WHERE tax_invoice_no = ?`, status, created.TaxInvoiceNo); err != nil {
			t.Fatalf("set reservation status %s failed: %v", status, err)
		}
		usage = requireTaxInvoiceUsage(t, service, "bill_a", "bill_b")
		if usage["bill_a"] != 400 || usage["bill_b"] != 300 {
			t.Fatalf("status %s released capacity: %v", status, usage)
		}
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_compliance_tax_invoice
SET status_code = 'rejected', decision_reason_code = 'duplicate_request'
WHERE tax_invoice_no = ?`, created.TaxInvoiceNo); err != nil {
		t.Fatalf("reject request failed: %v", err)
	}
	if usage = requireTaxInvoiceUsage(t, service, "bill_a", "bill_b"); len(usage) != 0 {
		t.Fatalf("explicit rejection did not release capacity: %v", usage)
	}
	cancelInput := input
	cancelInput.IdempotencyKey = "invoice-request-000002"
	cancelCreated, err := service.CreateCurrentCustomer(context.Background(), cancelInput)
	if err != nil {
		t.Fatalf("create request for cancellation usage failed: %v", err)
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_compliance_tax_invoice
SET status_code = 'canceled', decision_reason_code = 'customer_requested_cancellation'
WHERE tax_invoice_no = ?`, cancelCreated.TaxInvoiceNo); err != nil {
		t.Fatalf("cancel request failed: %v", err)
	}
	if usage = requireTaxInvoiceUsage(t, service, "bill_a", "bill_b"); len(usage) != 0 {
		t.Fatalf("explicit cancellation did not release capacity: %v", usage)
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_compliance_tax_invoice
SET customer_no = 'customer_2' WHERE tax_invoice_no = ?`, created.TaxInvoiceNo); err == nil {
		t.Fatal("immutable request customer was changed")
	}
	if _, err = database.Exec(context.Background(), `UPDATE plugin_idc_compliance_tax_invoice_item
SET gross_amount_minor = gross_amount_minor + 1 WHERE tax_invoice_no = ?`, created.TaxInvoiceNo); err == nil {
		t.Fatal("immutable request item was changed")
	}
	if _, err = database.Exec(context.Background(), `DELETE FROM plugin_idc_compliance_tax_invoice
WHERE tax_invoice_no = ?`, created.TaxInvoiceNo); err == nil {
		t.Fatal("tax invoice legal record was deleted")
	}
}

// TestCreateCurrentCustomerConcurrentCapacityPostgreSQL proves two different
// request keys cannot both reserve more than one source invoice allows.
func TestCreateCurrentCustomerConcurrentCapacityPostgreSQL(t *testing.T) {
	database := newTaxInvoiceRequestPostgresDatabase(t)
	store := newMemorySecretStore()
	service, err := New(database, &postgresSource{database: database, customerNo: "customer_1"}, store, store, &taxInvoiceCustomer{customerNo: "customer_1"})
	if err != nil {
		t.Fatalf("construct concurrent request service failed: %v", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func(index int) {
			<-start
			_, createErr := service.CreateCurrentCustomer(context.Background(), CreateInput{
				ProfileNo: "profile_1", IdempotencyKey: fmt.Sprintf("concurrent-key-%08d", index),
				Selections: []taxinvoicecap.Selection{{
					BillingInvoiceNo: "bill_a", RequestedGrossMinor: 700,
				}},
			})
			results <- createErr
		}(index)
	}
	close(start)
	var succeeded, failed int
	for index := 0; index < 2; index++ {
		if createErr := <-results; createErr == nil {
			succeeded++
		} else if bizerr.Is(createErr, CodeInvalid) {
			failed++
		} else {
			t.Fatalf("unexpected concurrent error: %v", createErr)
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("concurrent results succeeded=%d failed=%d", succeeded, failed)
	}
	usage := requireTaxInvoiceUsage(t, service, "bill_a")
	if usage["bill_a"] != 700 {
		t.Fatalf("concurrent reserved usage=%v want 700", usage)
	}
}

func requireTaxInvoiceUsage(t *testing.T, service Service, numbers ...string) map[string]int64 {
	t.Helper()
	rows, err := service.ReadAllocatedUsage(context.Background(), "customer_1", numbers)
	if err != nil {
		t.Fatalf("read reserved usage failed: %v", err)
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.BillingInvoiceNo] = row.AllocatedGrossMinor
	}
	return result
}

func newTaxInvoiceRequestPostgresDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(taxInvoiceRequestPostgresDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run tax invoice request PostgreSQL tests", taxInvoiceRequestPostgresDSNEnvironment)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create tax invoice request admin database failed: %v", err)
	}
	schemaDigest := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())))
	schema := "idc_tax_invoice_test_" + hex.EncodeToString(schemaDigest[:8])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create tax invoice request schema failed: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "idc_tax_invoice_test_") {
			if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); dropErr != nil {
				t.Errorf("drop tax invoice request schema failed: %v", dropErr)
			}
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close tax invoice request admin database failed: %v", closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("create scoped tax invoice request database failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped tax invoice request database failed: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    tenant_id INT NOT NULL,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(128) NOT NULL,
    status INT NOT NULL,
    is_builtin INT NOT NULL,
    allow_tenant_override BOOLEAN NOT NULL,
    remark VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    tenant_id INT NOT NULL,
    dict_type VARCHAR(128) NOT NULL,
    label VARCHAR(256) NOT NULL,
    value VARCHAR(128) NOT NULL,
    sort INT NOT NULL,
    tag_style VARCHAR(32) NOT NULL,
    status INT NOT NULL,
    is_builtin INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (tenant_id, dict_type, value)
);`); err != nil {
		t.Fatalf("create tax invoice dictionary fixtures failed: %v", err)
	}
	for _, name := range []string{"001-compliance-foundation.sql", "019-tax-invoice-request.sql"} {
		payload, readErr := os.ReadFile(filepath.Join(taxInvoiceRequestPluginRoot(t), "manifest", "sql", name))
		if readErr != nil {
			t.Fatalf("read migration %s failed: %v", name, readErr)
		}
		if _, execErr := database.Exec(ctx, string(payload)); execErr != nil {
			t.Fatalf("execute migration %s failed: %v", name, execErr)
		}
	}
	if _, err = database.Exec(ctx, `CREATE TABLE test_tax_invoice_source (
invoice_no VARCHAR(64) PRIMARY KEY,
order_no VARCHAR(64) NOT NULL,
item_no VARCHAR(64) NOT NULL UNIQUE,
gross_minor BIGINT NOT NULL,
net_minor BIGINT NOT NULL,
tax_minor BIGINT NOT NULL
);
INSERT INTO test_tax_invoice_source
(invoice_no, order_no, item_no, gross_minor, net_minor, tax_minor)
VALUES
('bill_a', 'order_a', 'bill_item_a', 1000, 900, 100),
('bill_b', 'order_b', 'bill_item_b', 1000, 900, 100);`); err != nil {
		t.Fatalf("create tax invoice source fixture failed: %v", err)
	}
	return database
}

func taxInvoiceRequestPluginRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve tax invoice request test path failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
