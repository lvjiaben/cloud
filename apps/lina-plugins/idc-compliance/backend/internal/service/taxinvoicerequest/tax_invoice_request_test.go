// This file verifies deterministic customer input and immutable snapshot encoding.

package taxinvoicerequest

import (
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// TestNormalizeInputAndIdentity proves browser array order cannot create a
// second idempotent identity and changed amounts cannot reuse the first digest.
func TestNormalizeInputAndIdentity(t *testing.T) {
	left, err := normalizeInput(CreateInput{
		ProfileNo: "profile_1", IdempotencyKey: "request-key-00000001",
		Selections: []taxinvoicecap.Selection{
			{BillingInvoiceNo: "bill_b", RequestedGrossMinor: 200},
			{BillingInvoiceNo: "bill_a", RequestedGrossMinor: 100},
		},
	})
	if err != nil {
		t.Fatalf("normalize first request failed: %v", err)
	}
	right, err := normalizeInput(CreateInput{
		ProfileNo: "profile_1", IdempotencyKey: "request-key-00000001",
		Selections: []taxinvoicecap.Selection{
			{BillingInvoiceNo: "bill_a", RequestedGrossMinor: 100},
			{BillingInvoiceNo: "bill_b", RequestedGrossMinor: 200},
		},
	})
	if err != nil {
		t.Fatalf("normalize reordered request failed: %v", err)
	}
	leftNo, leftDigest, err := deriveRequestIdentity("customer_1", left)
	if err != nil {
		t.Fatalf("derive first identity failed: %v", err)
	}
	rightNo, rightDigest, err := deriveRequestIdentity("customer_1", right)
	if err != nil {
		t.Fatalf("derive reordered identity failed: %v", err)
	}
	if leftNo != rightNo || leftDigest != rightDigest {
		t.Fatalf("reordered request changed identity: %q/%q versus %q/%q", leftNo, leftDigest, rightNo, rightDigest)
	}
	right.Selections[0].RequestedGrossMinor++
	_, changedDigest, err := deriveRequestIdentity("customer_1", right)
	if err != nil {
		t.Fatalf("derive changed identity failed: %v", err)
	}
	if changedDigest == leftDigest {
		t.Fatal("changed amount reused the original request digest")
	}
}

// TestNewSnapshotBundleSeparatesHeaderAndItemDigests proves every immutable
// source fact contributes to the encrypted bundle and its database digest.
func TestNewSnapshotBundleSeparatesHeaderAndItemDigests(t *testing.T) {
	snapshot := taxinvoicecap.RequestSnapshot{
		CustomerNo: "customer_1", PrincipalReference: "principal_1", ActorUserID: 1,
		Buyer: taxinvoicecap.ProfileSnapshot{ProfileSummary: taxinvoicecap.ProfileSummary{
			ProfileNo: "profile_1", TitleTypeCode: "enterprise", InvoiceTitle: "Buyer",
			TaxRegistrationNo: "tax-buyer", Version: 1,
		}},
		Seller: taxinvoicecap.MerchantSnapshot{
			MerchantNo: "merchant_1", LegalName: "Seller", CountryCode: "CN", Version: 1,
		},
		CurrencyCode: "CNY", GrossAmountMinor: 1000, NetAmountMinor: 900,
		TaxAmountMinor: 100, ObservedAt: time.Now().UTC(),
		Invoices: []taxinvoicecap.InvoiceSnapshot{{
			InvoiceOption: taxinvoicecap.InvoiceOption{
				BillingInvoiceNo: "bill_1", OrderNo: "order_1", CurrencyCode: "CNY",
			},
			RequestedGrossMinor: 1000, NetAmountMinor: 900, TaxAmountMinor: 100,
			Items: []taxinvoicecap.ItemSnapshot{{
				BillingInvoiceItemNo: "bill_item_1", SourceReference: "order_item_1",
				DescriptionZhCN: "服务器", DescriptionEnUS: "Server",
				TaxTreatmentCode: "taxable", TaxRatePPM: 111111,
				TaxClassificationCode: "server_service", GrossAmountMinor: 1000,
				NetAmountMinor: 900, TaxAmountMinor: 100,
				SourceContentDigest: digestText("source\x00", "bill_item_1"),
			}},
		}},
	}
	bundle, digests, payload, err := newSnapshotBundle("TINV_0123456789ABCDEF0123456789ABCDEF", &snapshot)
	if err != nil {
		t.Fatalf("create snapshot bundle failed: %v", err)
	}
	defer clear(payload)
	if bundle == nil || len(payload) == 0 || len(digests.Bundle) != 64 ||
		len(digests.Seller) != 64 || len(digests.Buyer) != 64 ||
		len(digests.Items["bill_item_1"]) != 64 {
		t.Fatalf("snapshot digests are incomplete: %#v", digests)
	}
	if digests.Seller == digests.Buyer || digests.Bundle == digests.Items["bill_item_1"] {
		t.Fatal("separate snapshot purposes unexpectedly shared one digest")
	}
}
