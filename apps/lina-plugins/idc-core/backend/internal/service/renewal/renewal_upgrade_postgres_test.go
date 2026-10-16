// This file checks modern upgrades against the fully migrated PostgreSQL schema.
// Catalog, checkout, quote freezing, invoice and effective price are real; only
// the supplier boundary is replaced, and no external supplier is contacted.
package renewal

import (
	"context"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/entitlementupgrade"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"strings"
	"testing"
	"time"
)

type upgradeProviderFixture struct {
	creates int
	status  serveroperationresourcecap.TaskStatus
}

func (s *upgradeProviderFixture) CompileUpgrade(_ context.Context, r serveroperationresourcecap.UpgradeConfigurationRequest) (*serveroperationresourcecap.UpgradeConfiguration, error) {
	return &serveroperationresourcecap.UpgradeConfiguration{Route: serveroperationresourcecap.EntitlementRoute{ServiceNo: r.ServiceNo, ChannelNo: "channel_upgrade", PoolNo: "pool_upgrade"}, Parameters: []byte(`{"cpu":4}`)}, nil
}
func (s *upgradeProviderFixture) CreateTask(context.Context, serveroperationresourcecap.TaskRequest) (*serveroperationresourcecap.Task, error) {
	s.creates++
	return s.result(), nil
}
func (s *upgradeProviderFixture) ReadTask(context.Context, serveroperationresourcecap.TaskReadRequest) (*serveroperationresourcecap.Task, error) {
	return s.result(), nil
}
func (s *upgradeProviderFixture) result() *serveroperationresourcecap.Task {
	now := time.Now()
	out := &serveroperationresourcecap.Task{ProviderTaskNo: "task_upgrade", Status: s.status, CapabilityEvidenceNo: "cap_upgrade", CapabilityEvidenceDigest: strings.Repeat("a", 64), EvidenceDigest: strings.Repeat("b", 64), ObservedAt: now, CompletedAt: now}
	if s.status == serveroperationresourcecap.TaskStatusFailed || s.status == serveroperationresourcecap.TaskStatusResultUnknown {
		out.FailureCode = "provider_test_failure"
	}
	if s.status == serveroperationresourcecap.TaskStatusResultUnknown {
		out.CompletedAt = time.Time{}
	}
	return out
}

func TestEntitlementUpgradeRealSchemaPostgreSQL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		paid   bool
		status serveroperationresourcecap.TaskStatus
	}{
		{"free_success", false, serveroperationresourcecap.TaskStatusSucceeded},
		{"paid_success", true, serveroperationresourcecap.TaskStatusSucceeded},
		{"paid_generic", true, serveroperationresourcecap.TaskStatusSucceeded},
		{"expired_unpaid", true, serveroperationresourcecap.TaskStatusSucceeded},
		{"paid_failure", true, serveroperationresourcecap.TaskStatusFailed},
		{"paid_unknown", true, serveroperationresourcecap.TaskStatusResultUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runUpgradeSchema(t, tc.paid, tc.status, tc.name == "paid_generic", tc.name == "expired_unpaid")
		})
	}
}
func runUpgradeSchema(t *testing.T, paid bool, providerStatus serveroperationresourcecap.TaskStatus, genericPayment, expiredUnpaid bool) {
	ctx, db := newRenewalPostgreSQLDatabase(t)
	reader, owner, fixture := newGenericRenewalCheckout(t, ctx, db)
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET entitlement_owner_id='idc-resource',status_code='active',started_at=CURRENT_TIMESTAMP-INTERVAL '15 days',expires_at=CURRENT_TIMESTAMP+INTERVAL '15 days' WHERE entitlement_no=?`, fixture.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if paid {
		if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_catalog_price SET amount_minor=amount_minor+5000 WHERE price_no='PRC_renewal_ssl_base'`); err != nil {
			t.Fatal(err)
		}
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	cartSvc, err := commerce.New(db, resolver, owner, renewalPostgreSQLSettlementStub{})
	if err != nil {
		t.Fatal(err)
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(db, owner)
	if err != nil {
		t.Fatal(err)
	}
	if err = commerce.BindStructuredConfigurationSnapshots(cartSvc, freezer, files); err != nil {
		t.Fatal(err)
	}
	config, err := reader.GetPurchaseConfiguration(ctx, "renewal-ssl-certificate")
	if err != nil {
		t.Fatal(err)
	}
	values := []catalog.PublicPurchaseConfigurationValue{{FieldToken: config.Fields[0].FieldToken, Type: "single_select", OptionTokens: []string{config.Fields[0].Options[0].OptionToken}}}
	priced, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "renewal-ssl-certificate", ConfigurationRevision: config.ConfigurationRevision, Values: values}, CurrencyCode: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	cart, err := cartSvc.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	cart, err = cartSvc.AddAccountItem(ctx, commerce.AddItemInput{ExpectedCartVersion: cart.Version, Quantity: 1, Purchase: catalog.PurchaseConfigurationInput{PublicSlug: "renewal-ssl-certificate", ConfigurationRevision: config.ConfigurationRevision, BillingTermToken: priced.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1, Values: values}})
	if err != nil {
		t.Fatal(err)
	}
	quote, err := cartSvc.QuoteAccountCart(ctx, commerce.QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cartSvc.FreezeAccountQuoteItemConfiguration(ctx, commerce.FreezeQuoteItemConfigurationInput{QuoteItemNo: quote.Items[0].QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: "renewal-ssl-certificate", ConfigurationRevision: config.ConfigurationRevision, Values: values}}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billing.NewInvoiceService(db)
	if err != nil {
		t.Fatal(err)
	}
	provider := &upgradeProviderFixture{status: providerStatus}
	runtime, err := entitlementupgrade.New(db, owner, invoices, provider)
	if err != nil {
		t.Fatal(err)
	}
	list, err := runtime.List(ctx)
	if err != nil || len(list.Targets) != 1 {
		t.Fatalf("discovery=%+v err=%+v", list, err)
	}
	original := owner.audience.CustomerNo
	owner.audience.CustomerNo = "other_customer"
	if _, err = runtime.Quote(ctx, fixture.EntitlementNo, quote.QuoteNo, "upgrade-schema-12345"); err == nil {
		t.Fatal("foreign quote accepted")
	}
	owner.audience.CustomerNo = original
	upgrade, err := runtime.Quote(ctx, fixture.EntitlementNo, quote.QuoteNo, "upgrade-schema-12345")
	if err != nil {
		t.Fatalf("quote: %+v", err)
	}
	replay, err := runtime.Quote(ctx, fixture.EntitlementNo, quote.QuoteNo, "upgrade-schema-12345")
	if err != nil || replay.UpgradeNo != upgrade.UpgradeNo {
		t.Fatalf("quote replay=%+v err=%+v", replay, err)
	}
	confirmed, err := runtime.Confirm(ctx, upgrade.UpgradeNo)
	if err != nil {
		t.Fatalf("confirmation: %+v", err)
	}
	if expiredUnpaid {
		if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_entitlement_upgrade SET quote_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE upgrade_no=?`, upgrade.UpgradeNo); err != nil {
			t.Fatal(err)
		}
		worker, workerErr := entitlementupgrade.NewWorker(db, invoices, provider)
		if workerErr != nil {
			t.Fatal(workerErr)
		}
		if _, err = worker.Run(ctx, 10); err != nil {
			t.Fatal(err)
		}
		if _, err = worker.Run(ctx, 10); err != nil {
			t.Fatal(err)
		}
		state, stateErr := runtime.List(ctx)
		invoice, invoiceErr := db.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_billing_invoice WHERE invoice_no=?`, confirmed.InvoiceNo)
		if stateErr != nil || invoiceErr != nil || len(state.Items) != 1 || state.Items[0].StatusCode != "cancelled" || invoice.String() != "cancelled" || provider.creates != 0 {
			t.Fatalf("expired quote not safely released: %+v invoice=%v creates=%d errors=%v/%v", state, invoice, provider.creates, stateErr, invoiceErr)
		}
		return
	}
	if paid {
		if confirmed.StatusCode != "pending_payment" || confirmed.TotalMinor < 2499 || confirmed.TotalMinor > 2500 {
			t.Fatalf("paid amount=%+v", confirmed)
		}
		if _, err = runtime.Pay(ctx, upgrade.UpgradeNo); err == nil {
			t.Fatal("unfunded payment accepted")
		}
		if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='prd_renewal_ssl'`); err != nil {
			t.Fatal(err)
		}
		if _, err = runtime.Pay(ctx, upgrade.UpgradeNo); err == nil {
			t.Fatal("stopped product payment accepted")
		}
		if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='active' WHERE product_no='prd_renewal_ssl'`); err != nil {
			t.Fatal(err)
		}
		balance, err := billing.NewBalanceService(db)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = balance.PostConfirmedRecharge(ctx, billing.ConfirmedRechargeInput{CustomerNo: owner.audience.CustomerNo, CurrencyCode: "CNY", AmountMinor: 50000, SourceReference: "upgrade-recharge", SourceSnapshotDigest: strings.Repeat("a", 64)}); err != nil {
			t.Fatal(err)
		}
		owner.audience.CustomerNo = "other_customer"
		if _, err = runtime.Pay(ctx, upgrade.UpgradeNo); err == nil {
			t.Fatal("foreign payment accepted")
		}
		owner.audience.CustomerNo = original
		if genericPayment {
			digest, readErr := db.GetValue(ctx, `SELECT content_digest FROM plugin_idc_core_entitlement_upgrade WHERE upgrade_no=?`, upgrade.UpgradeNo)
			if readErr != nil {
				t.Fatal(readErr)
			}
			input := billing.InvoiceBalancePaymentInput{InvoiceNo: confirmed.InvoiceNo, SourceReference: "generic-upgrade-payment", SourceSnapshotDigest: digest.String()}
			if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='prd_renewal_ssl'`); err != nil {
				t.Fatal(err)
			}
			if _, err = invoices.PayFromBalance(ctx, input); err == nil {
				t.Fatal("generic payment bypassed stopped product guard")
			}
			if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='active' WHERE product_no='prd_renewal_ssl'`); err != nil {
				t.Fatal(err)
			}
			if _, err = invoices.PayFromBalance(ctx, input); err != nil {
				t.Fatalf("generic payment: %+v", err)
			}
		}
		if _, err = runtime.Pay(ctx, upgrade.UpgradeNo); err != nil {
			t.Fatalf("pay: %+v", err)
		}
		if _, err = runtime.Pay(ctx, upgrade.UpgradeNo); err != nil {
			t.Fatal(err)
		}
		count, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_billing_payment_allocation WHERE invoice_no=?`, confirmed.InvoiceNo)
		if err != nil || count.Int() != 1 {
			t.Fatalf("duplicate debit allocations=%v err=%v", count, err)
		}
	} else if confirmed.StatusCode != "ready" || confirmed.TotalMinor != 0 {
		t.Fatalf("zero amount upgrade=%+v", confirmed)
	}
	if _, err = runtime.Run(ctx, 10); err != nil {
		t.Fatalf("worker: %+v", err)
	}
	if _, err = runtime.Run(ctx, 10); err != nil {
		t.Fatal(err)
	}
	state, err := runtime.List(ctx)
	if err != nil || len(state.Items) != 1 || state.Items[0].StatusCode != func() string {
		switch providerStatus {
		case serveroperationresourcecap.TaskStatusFailed:
			return "retry_wait"
		case serveroperationresourcecap.TaskStatusResultUnknown:
			return "manual_review"
		default:
			return "succeeded"
		}
	}() || provider.creates != 1 {
		t.Fatalf("completed=%+v creates=%d err=%+v", state, provider.creates, err)
	}
	price, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_service_price_snapshot p ON p.price_snapshot_no=e.current_price_snapshot_no WHERE e.entitlement_no=? AND p.source_quote_no=? AND p.structured_configuration_snapshot_no IS NOT NULL`, fixture.EntitlementNo, quote.QuoteNo)
	want := 1
	if providerStatus != serveroperationresourcecap.TaskStatusSucceeded {
		want = 0
	}
	if err != nil || price.Int() != want {
		t.Fatalf("effective price=%v err=%v", price, err)
	}
	if want == 1 {
		facts, err := lockOwnedEntitlementForQuote(ctx, db, original, fixture.EntitlementNo)
		if err != nil || facts == nil || facts.PriceConfigurationSnapshotNo == fixture.PriceConfigurationSnapshotNo {
			t.Fatalf("renewal did not follow upgraded price: %+v err=%v", facts, err)
		}
	}
	if providerStatus == serveroperationresourcecap.TaskStatusFailed {
		// Advance only this isolated fixture's scheduling time, never the clock.
		for attempt := 0; attempt < 2; attempt++ {
			if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_entitlement_upgrade SET available_at=CURRENT_TIMESTAMP WHERE upgrade_no=?`, upgrade.UpgradeNo); err != nil {
				t.Fatal(err)
			}
			if _, err = runtime.Run(ctx, 10); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = runtime.Run(ctx, 10); err != nil {
			t.Fatal(err)
		}
		final, readErr := runtime.List(ctx)
		if readErr != nil || len(final.Items) != 1 || final.Items[0].StatusCode != "manual_review" || provider.creates != 3 {
			t.Fatalf("failure retries were not capped: %+v creates=%d err=%v", final, provider.creates, readErr)
		}
	}
}
