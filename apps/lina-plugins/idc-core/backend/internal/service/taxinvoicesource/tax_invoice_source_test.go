// This file verifies deterministic tax-invoice source validation and allocation helpers.

package taxinvoicesource

import (
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

func TestAllocateInvoiceItemsUsesServerTaxFactsAndLastItemRemainder(t *testing.T) {
	rate := 600
	rows := []itemRow{
		{
			InvoiceNo: "invoice-1", InvoiceItemNo: "item-1", SourceReference: "order-item-1",
			DescriptionZhCN: "服务器一", DescriptionEnUS: "Server One",
			NetAmountMinor: 566, TaxAmountMinor: 34, TotalAmountMinor: 600,
			RefundedAmountMinor: 100, TaxTreatmentCode: "taxable",
			TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service",
			SourceContentDigest: "digest-1",
		},
		{
			InvoiceNo: "invoice-1", InvoiceItemNo: "item-2", SourceReference: "order-item-2",
			DescriptionZhCN: "服务器二", DescriptionEnUS: "Server Two",
			NetAmountMinor: 377, TaxAmountMinor: 23, TotalAmountMinor: 400,
			TaxTreatmentCode: "taxable", TaxRateBasisPoints: &rate,
			TaxCategoryCode: "cloud_service", SourceContentDigest: "digest-2",
		},
	}
	usage := taxinvoicecap.InvoiceUsage{
		BillingInvoiceNo: "invoice-1", AllocatedGrossMinor: 100,
		Items: []taxinvoicecap.ItemUsage{{
			BillingInvoiceItemNo: "item-1", AllocatedGrossMinor: 100,
			AllocatedNetMinor: 94, AllocatedTaxMinor: 6,
		}},
	}
	items, net, tax, err := allocateInvoiceItems(rows, usage, 800, 401)
	if err != nil {
		t.Fatalf("allocate invoice items: %v", err)
	}
	if len(items) != 2 || items[0].GrossAmountMinor != 200 ||
		items[1].GrossAmountMinor != 201 {
		t.Fatalf("unexpected deterministic gross allocation: %+v", items)
	}
	if net != 377 || tax != 24 || net+tax != 401 {
		t.Fatalf("unexpected tax allocation: net=%d tax=%d", net, tax)
	}
	if items[0].TaxRatePPM != 60000 || items[0].TaxClassificationCode != "cloud_service" {
		t.Fatalf("source tax facts were not preserved: %+v", items[0])
	}
}

func TestAllocateInvoiceItemsRejectsUsageOutsideSource(t *testing.T) {
	rate := 600
	rows := []itemRow{{
		InvoiceNo: "invoice-1", InvoiceItemNo: "item-1", NetAmountMinor: 94,
		TaxAmountMinor: 6, TotalAmountMinor: 100, TaxTreatmentCode: "taxable",
		TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service",
		SourceContentDigest: "digest-1",
	}}
	usage := taxinvoicecap.InvoiceUsage{
		BillingInvoiceNo: "invoice-1", AllocatedGrossMinor: 101,
		Items: []taxinvoicecap.ItemUsage{{
			BillingInvoiceItemNo: "item-1", AllocatedGrossMinor: 101,
			AllocatedNetMinor: 95, AllocatedTaxMinor: 6,
		}},
	}
	if _, _, _, err := allocateInvoiceItems(rows, usage, -1, 1); err == nil {
		t.Fatal("allocation usage beyond the core-owned item was accepted")
	}
}

func TestNormalizeSelectionsRejectsDuplicatesAndSortsLocks(t *testing.T) {
	requested, invoiceNos, err := normalizeSelections([]taxinvoicecap.Selection{
		{BillingInvoiceNo: "invoice-2", RequestedGrossMinor: 20},
		{BillingInvoiceNo: "invoice-1", RequestedGrossMinor: 10},
	})
	if err != nil || len(requested) != 2 || invoiceNos[0] != "invoice-1" || invoiceNos[1] != "invoice-2" {
		t.Fatalf("selection normalization failed: requested=%v invoiceNos=%v err=%v", requested, invoiceNos, err)
	}
	if _, _, err = normalizeSelections([]taxinvoicecap.Selection{
		{BillingInvoiceNo: "invoice-1", RequestedGrossMinor: 10},
		{BillingInvoiceNo: "invoice-1", RequestedGrossMinor: 10},
	}); err == nil {
		t.Fatal("duplicate billing invoice selection was accepted")
	}
}

func TestPublishedRefundOperationNoKeepsSourceNamespacesSeparate(t *testing.T) {
	manual, err := publishedRefundOperationNo("manual", "same-reference")
	if err != nil {
		t.Fatal(err)
	}
	balance, err := publishedRefundOperationNo("balance_credit", "same-reference")
	if err != nil {
		t.Fatal(err)
	}
	if manual == balance {
		t.Fatalf("refund source namespaces collapsed to %q", manual)
	}
	sourceType, sourceReference, err := parsePublishedRefundOperationNo(balance)
	if err != nil || sourceType != "balance_credit" || sourceReference != "same-reference" {
		t.Fatalf("parsed namespace=%q reference=%q err=%v", sourceType, sourceReference, err)
	}
	if _, _, err = parsePublishedRefundOperationNo("same-reference"); err == nil {
		t.Fatal("refund operation without a source namespace was accepted")
	}
}

func TestValidateItemRowRejectsMissingSourceIdentity(t *testing.T) {
	rate := 600
	row := testItem("invoice-1", "item-1", &rate)
	row.DescriptionZhCN = "服务器"
	if err := validateItemRow(&row); err == nil {
		t.Fatal("billing item without a source reference was accepted")
	}
	row.SourceReference = "order-item-1"
	row.DescriptionZhCN = ""
	row.DescriptionEnUS = ""
	if err := validateItemRow(&row); err == nil {
		t.Fatal("billing item without a description was accepted")
	}
}

func TestBuildRequestSnapshotRejectsMixedCurrencies(t *testing.T) {
	rate := 600
	subject := testCustomerSubject()
	profile := testProfile()
	merchant := &merchantRow{MerchantNo: "merchant-1", LegalName: "本地商家", CountryCode: "CN", Version: 1}
	invoices := []invoiceRow{
		testInvoice("invoice-1", "CNY", 100),
		testInvoice("invoice-2", "USD", 100),
	}
	items := []itemRow{
		testItem("invoice-1", "item-1", &rate),
		testItem("invoice-2", "item-2", &rate),
	}
	_, err := buildRequestSnapshot(
		subject, profile, merchant, invoices, items,
		map[string]taxinvoicecap.InvoiceUsage{},
		map[string]int64{"invoice-1": 100, "invoice-2": 100},
		testTime(),
	)
	if err == nil {
		t.Fatal("mixed-currency tax invoice request was accepted")
	}
}

func testCustomerSubject() *identitycap.CustomerSubject {
	return &identitycap.CustomerSubject{
		CustomerNo: "customer-1", CustomerTypeCode: "enterprise",
		PrincipalReference: "usr:7", ActorUserID: 7,
	}
}

func testProfile() *profileRow {
	return &profileRow{
		ProfileNo: "profile-1", TitleTypeCode: "enterprise",
		InvoiceTitle: "客户企业", TaxRegistrationNo: "913000000000000000",
		Version: 1,
	}
}

func testInvoice(invoiceNo, currency string, amount int64) invoiceRow {
	settledAt := testTime()
	return invoiceRow{
		InvoiceNo: invoiceNo, CurrencyCode: currency, PaidAmountMinor: amount,
		ContentDigest: "digest-" + invoiceNo, SettledAt: &settledAt, Version: 1,
	}
}

func testItem(invoiceNo, itemNo string, rate *int) itemRow {
	return itemRow{
		InvoiceNo: invoiceNo, InvoiceItemNo: itemNo, NetAmountMinor: 94,
		TaxAmountMinor: 6, TotalAmountMinor: 100, TaxTreatmentCode: "taxable",
		TaxRateBasisPoints: rate, TaxCategoryCode: "cloud_service",
		SourceContentDigest: "digest-" + itemNo,
	}
}

func testTime() time.Time {
	return time.Date(2026, time.August, 17, 10, 0, 0, 0, time.UTC)
}
