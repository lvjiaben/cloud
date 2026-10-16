// This file verifies that the channel orchestration shares one PostgreSQL
// transaction with checkout/renewal owners and rolls back rejected payments.
package backend

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

type atomicCheckout struct {
	commerce.ChannelOrderService
	db gdb.DB
}

func (s atomicCheckout) Checkout(ctx context.Context, _ commerce.ChannelOrderInput) (*commerce.Checkout, error) {
	err := s.db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := s.db.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice VALUES(0,'invoice','order','customer')`)
		return err
	})
	return &commerce.Checkout{OrderNo: "order", InvoiceNo: "invoice", TotalAmountMinor: 100}, err
}
func (atomicCheckout) Quote(context.Context, string, catalog.PurchaseConfigurationInput, int) (*commerce.Quote, error) {
	return nil, nil
}

type insufficientInvoice struct{ billing.InvoiceService }

func (insufficientInvoice) PayFromBalance(context.Context, billing.InvoiceBalancePaymentInput) (*billing.InvoiceBalancePayment, error) {
	return nil, bizerr.NewCode(billing.CodeInsufficientBalance)
}

type atomicRenewal struct {
	renewal.RuntimeService
	db gdb.DB
}

func (s atomicRenewal) QuoteForChannel(ctx context.Context, _ string, _ renewal.QuoteInput) (*renewal.Quote, error) {
	_, err := s.db.Exec(ctx, `INSERT INTO renewal_evidence VALUES('quote')`)
	return &renewal.Quote{QuoteNo: "quote"}, err
}
func (s atomicRenewal) ConfirmForChannel(ctx context.Context, _ string, _ string, _ renewal.ConfirmInput, _ string) (*renewal.Operation, error) {
	_, err := s.db.Exec(ctx, `INSERT INTO renewal_evidence VALUES('operation')`)
	return &renewal.Operation{OperationNo: "operation", TotalAmountMinor: 100, Status: "awaiting_financial_settlement"}, err
}
func (atomicRenewal) PayForChannel(context.Context, string, string, renewal.PayInput, string, string) (*renewal.Settlement, error) {
	return nil, bizerr.NewCode(renewal.CodeBalanceInsufficient)
}

func TestChannelRejectedPaymentsRollbackPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skip("PostgreSQL DSN required")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newCatalogIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	_, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_customer_subject(tenant_id int,customer_no text,customer_type_code text,status_code text);
 CREATE TABLE plugin_idc_core_customer_enterprise_profile(tenant_id int,customer_no text,verification_status_code text);
 CREATE TABLE plugin_idc_core_customer_individual_profile(tenant_id int,customer_no text,identity_status_code text);
 CREATE TABLE plugin_idc_core_customer_identity_binding(id int,tenant_id int,customer_no text,sys_user_id int,status_code text);
 CREATE TABLE plugin_idc_core_billing_invoice(tenant_id int,invoice_no text,order_no text,customer_no text);
 CREATE TABLE renewal_evidence(kind text);
 INSERT INTO plugin_idc_core_customer_subject VALUES(0,'customer','individual','active');
 INSERT INTO plugin_idc_core_customer_individual_profile VALUES(0,'customer','verified');
 INSERT INTO plugin_idc_core_customer_identity_binding VALUES(1,0,'customer',42,'active');`)
	if err != nil {
		t.Fatal(err)
	}
	runtime := &channelCommerceRuntime{database: db, orders: atomicCheckout{db: db}, invoices: insufficientInvoice{}, renewals: atomicRenewal{db: db}}
	// Credential eligibility must explain the missing customer prerequisite,
	// rather than returning an unrelated financial-data permission error.
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_customer_individual_profile SET identity_status_code='unverified'`); err != nil {
		t.Fatal(err)
	}
	if subject, validationErr := runtime.ValidatePayer(ctx, "customer"); validationErr != nil || subject.IdentityStatus != "unverified" {
		t.Fatalf("unverified customer credential eligibility = %+v %v", subject, validationErr)
	}
	if _, err = runtime.requirePayer(ctx, "customer"); !bizerr.Is(err, customer.CodeVerificationRequired) {
		t.Fatalf("unverified transaction error = %v", err)
	}
	if _, err = runtime.ValidatePayer(ctx, "missing"); !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("missing customer error = %v", err)
	}
	for _, statement := range []string{
		`UPDATE plugin_idc_core_customer_subject SET status_code='inactive'`,
		`UPDATE plugin_idc_core_customer_subject SET status_code='active'; UPDATE plugin_idc_core_customer_identity_binding SET status_code='inactive'`,
	} {
		if _, err = db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
		if _, err = runtime.ValidatePayer(ctx, "customer"); !bizerr.Is(err, customer.CodeStateDenied) {
			t.Fatalf("unavailable customer credential eligibility = %v", err)
		}
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_customer_identity_binding SET status_code='active'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_customer_individual_profile SET identity_status_code='verified'`); err != nil {
		t.Fatal(err)
	}
	_, err = runtime.CreateOrder(ctx, channelcommercecap.CreateOrderInput{ChannelID: "app_test", OwnershipMode: "channel_owned", PayerCustomerNo: "customer", ServiceOwnerCustomerNo: "customer", IdempotencyKey: "request-one", Purchase: channelcommercecap.Purchase{Quantity: 1}})
	if !bizerr.Is(err, billing.CodeInsufficientBalance) {
		t.Fatalf("expected insufficient balance, got %v", err)
	}
	count, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_billing_invoice`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("checkout survived payment rejection: %v %v", count, err)
	}
	_, err = runtime.renewChannelService(ctx, channelcommercecap.LifecycleInput{ChannelID: "app_test", IdempotencyKey: "renewal-one", Parameters: []byte(`{"renewalCycles":1}`)}, &channelServiceRow{ServiceNo: "service", ServiceOwnerCustomerNo: "customer", PayerCustomerNo: "customer", StatusCode: "active"})
	if !bizerr.Is(err, renewal.CodeBalanceInsufficient) {
		t.Fatalf("expected insufficient renewal balance, got %v", err)
	}
	count, err = db.GetValue(ctx, `SELECT COUNT(*) FROM renewal_evidence`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("renewal survived payment rejection: %v %v", count, err)
	}
}

// successfulRenewal is an explicit renewal-owner substitute: the channel
// orchestration must use the settled authoritative projection, including on replay.
type successfulRenewal struct {
	renewal.RuntimeService
	db gdb.DB
}

func (s successfulRenewal) QuoteForChannel(ctx context.Context, _ string, _ renewal.QuoteInput) (*renewal.Quote, error) {
	count, err := s.db.GetValue(ctx, `SELECT COUNT(*) FROM renewal_response_evidence`)
	return &renewal.Quote{QuoteNo: "quote_response", Reused: count.Int() > 0}, err
}
func (s successfulRenewal) ConfirmForChannel(ctx context.Context, _ string, _ string, _ renewal.ConfirmInput, _ string) (*renewal.Operation, error) {
	count, err := s.db.GetValue(ctx, `SELECT COUNT(*) FROM renewal_response_evidence`)
	status := "awaiting_financial_settlement"
	if count.Int() > 0 {
		status = "pending_provider"
	}
	return &renewal.Operation{OperationNo: "operation_response", QuoteNo: "quote_response", CurrencyCode: "CNY", TotalAmountMinor: 120, Status: status, Reused: count.Int() > 0}, err
}
func (s successfulRenewal) PayForChannel(ctx context.Context, _ string, _ string, _ renewal.PayInput, _ string, _ string) (*renewal.Settlement, error) {
	if _, err := s.db.Exec(ctx, `INSERT INTO renewal_response_evidence VALUES('operation_response',120)`); err != nil {
		return nil, err
	}
	return &renewal.Settlement{Operation: &renewal.Operation{OperationNo: "operation_response", QuoteNo: "quote_response", CurrencyCode: "CNY", TotalAmountMinor: 120, Status: "pending_provider"}}, nil
}

// TestChannelRenewalResponsePostgreSQL exercises the real orchestration's
// transaction and response mapping; full owner/balance behavior is also covered
// by the public two-instance protocol scenario, rather than claimed by this substitute.
func TestChannelRenewalResponsePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skip("PostgreSQL DSN required")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newCatalogIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	db, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if _, err = db.Exec(ctx, `CREATE TABLE renewal_response_evidence(operation_no TEXT PRIMARY KEY,amount BIGINT)`); err != nil {
		t.Fatal(err)
	}
	runtime := &channelCommerceRuntime{database: db, renewals: successfulRenewal{db: db}}
	input := channelcommercecap.LifecycleInput{ChannelID: "app_response", IdempotencyKey: "renew-response", Parameters: []byte(`{"renewalCycles":1}`)}
	service := &channelServiceRow{ServiceNo: "service_response", ServiceOwnerCustomerNo: "customer_response", PayerCustomerNo: "customer_response", StatusCode: "active"}
	first, err := runtime.renewChannelService(ctx, input, service)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := runtime.renewChannelService(ctx, input, service)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []*channelcommercecap.LifecycleResult{first, replay} {
		if result.QuoteNo != "quote_response" || result.CurrencyCode != "CNY" || result.AmountMinor != 120 || result.Status != "pending_provider" || result.PaymentRequired {
			t.Fatalf("authoritative renewal response lost: %+v", result)
		}
	}
	if first.OperationNo != replay.OperationNo || first.Reused || !replay.Reused {
		t.Fatal("response replay identity lost")
	}
	total, err := db.GetValue(ctx, `SELECT SUM(amount) FROM renewal_response_evidence`)
	if err != nil || total.Int64() != 120 {
		t.Fatalf("orchestration called settlement more than once: total=%v err=%v", total, err)
	}
}
