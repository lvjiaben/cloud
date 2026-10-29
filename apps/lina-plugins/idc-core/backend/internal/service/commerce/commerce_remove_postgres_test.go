// Regression coverage for cart projection after removal, re-add and guest merge.
package commerce

import (
	"crypto/rand"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"testing"
)

func TestCartRemovalPreservesRemainingConfigurationPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)

	publicReader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create public catalog reader: %v", err)
	}
	bindTestPublicBillingLabels(t, publicReader)
	phone, err := publicReader.GetBySlug(ctx, "generic-phone")
	if err != nil || phone == nil || phone.PublicSlug != "generic-phone" {
		t.Fatalf("generic phone must remain visible in the public catalog: product=%+v err=%v", phone, err)
	}
	beforeLegacyAttempt := commerceLegacyWriteCounts(t, ctx, database)
	phoneConfiguration, err := publicReader.GetConfiguration(ctx, "generic-phone")
	if err != nil || phoneConfiguration == nil || phoneConfiguration.ConfigurationMode != catalog.ConfigurationFixed ||
		len(phoneConfiguration.Groups) != 0 || len(phoneConfiguration.Rules) != 0 {
		t.Fatalf("generic phone fixed configuration=%+v err=%v", phoneConfiguration, err)
	}
	afterLegacyAttempt := commerceLegacyWriteCounts(t, ctx, database)
	if beforeLegacyAttempt != afterLegacyAttempt {
		t.Fatalf("rejected generic phone created legacy commerce or fulfillment data: before=%+v after=%+v", beforeLegacyAttempt, afterLegacyAttempt)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(publicReader)
	if err != nil {
		t.Fatalf("resolve purchase catalog boundary: %v", err)
	}
	configuration, err := publicReader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		t.Fatalf("load public configuration: %v", err)
	}
	quote, err := publicReader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil {
		t.Fatalf("load public quote: %v", err)
	}
	if quote == nil || len(quote.Terms) != 1 {
		t.Fatalf("load public quote result=%+v", quote)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quote.Terms[0].BillingTermToken, CurrencyCode: "CNY",
		PurchaseCycles: 1,
	}
	customerOne := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-1",
	}}
	service, err := newService(
		database, resolver, customerOne, rand.Reader, commerceSettlementStub{},
	)
	if err != nil {
		t.Fatalf("create commerce service: %v", err)
	}

	guest, err := service.CreateGuestCart(ctx, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	cart := guest.Cart
	for range 2 {
		cart, err = service.AddGuestItem(ctx, guest.Token, AddItemInput{ExpectedCartVersion: cart.Version, Quantity: 1, Purchase: purchase})
		if err != nil {
			t.Fatal(err)
		}
	}
	first, second := cart.Items[0], cart.Items[1]
	if first.PublicSlug != "commerce-service-cloud" || first.ConfigurationReference == "" || first.ConfigurationReference == second.ConfigurationReference {
		t.Fatalf("configuration identity/slug missing: %+v", cart.Items)
	}
	other, err := service.CreateGuestCart(ctx, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RemoveGuestItem(ctx, other.Token, first.CartItemNo, other.Cart.Version, first.Version); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign guest removal: %v", err)
	}
	if _, err = service.RemoveGuestItem(ctx, guest.Token, first.CartItemNo, cart.Version-1, first.Version); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale removal: %v", err)
	}
	cart, err = service.RemoveGuestItem(ctx, guest.Token, first.CartItemNo, cart.Version, first.Version)
	if err != nil || cart == nil || len(cart.Items) != 1 || cart.Items[0].CartItemNo != second.CartItemNo {
		t.Fatalf("remove while retaining another item: cart=%+v error=%v", cart, err)
	}
	cart, err = service.GetGuestCart(ctx, guest.Token)
	if err != nil || len(cart.Items) != 1 {
		t.Fatalf("reload after removal: %+v %v", cart, err)
	}
	cart, err = service.AddGuestItem(ctx, guest.Token, AddItemInput{ExpectedCartVersion: cart.Version, Quantity: 2, Purchase: purchase})
	if err != nil || len(cart.Items) != 2 {
		t.Fatalf("re-add after removal: %+v %v", cart, err)
	}
	merged, err := service.MergeGuestCart(ctx, guest.Token)
	if err != nil || len(merged.Items) != 2 {
		t.Fatalf("merge after removal: %+v %v", merged, err)
	}
	if merged.Items[0].CartItemNo == second.CartItemNo || merged.Items[0].ConfigurationReference != second.ConfigurationReference {
		t.Fatalf("merge must retain exact configuration reference: %+v", merged.Items)
	}
	removed, err := service.RemoveAccountItem(ctx, merged.Items[0].CartItemNo, merged.Version, merged.Items[0].Version)
	if err != nil || len(removed.Items) != 1 {
		t.Fatalf("account removal: %+v %v", removed, err)
	}
	if _, err = service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: removed.Version}); err != nil {
		t.Fatalf("quote remaining item after removal: %v", err)
	}
}
