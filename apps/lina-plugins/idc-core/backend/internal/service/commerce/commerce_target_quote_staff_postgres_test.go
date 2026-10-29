// Real staff targets preserve owner and actor separately and restrict freezing.
package commerce

import (
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"testing"
)

func TestStaffTargetQuoteAndExactFreezerPostgreSQL(t *testing.T) {
	f := newTargetQuoteFixture(t)
	staff := riskReviewPrincipalStub{audience: &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}}
	authorizer, err := NewTargetQuoteAuthorizer(riskReviewBizCtxStub{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 2}}, staff, riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionTargetQuoteEdit): true}})
	if err != nil {
		t.Fatal(err)
	}
	if err = BindTargetQuoteAuthorizer(f.service, authorizer); err != nil {
		t.Fatal(err)
	}
	input := AccountTargetQuoteInput{EntitlementNo: f.entitlementNo, Purchase: f.purchase}
	if _, err = f.service.QuoteStaffTarget(f.ctx, "customer-commerce-service-2", input); err == nil {
		t.Fatal("staff quoted a parent belonging to another customer")
	}
	quote, err := f.service.QuoteStaffTarget(f.ctx, "customer-commerce-service-1", input)
	if err != nil {
		t.Fatalf("staff target quote: %+v", err)
	}
	freeze := FreezeQuoteItemConfigurationInput{QuoteItemNo: quote.Items[0].QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: f.purchase.ConfigurationRevision, Values: f.purchase.Values}}
	if _, err = f.service.FreezeStaffTargetQuoteItemConfiguration(f.ctx, "customer-commerce-service-1", "ENT_wrong_parent", freeze); err == nil {
		t.Fatal("staff froze a different parent")
	}
	original := f.service.targetQuoteAuthorizer
	other, err := NewTargetQuoteAuthorizer(riskReviewBizCtxStub{current: bizctxcap.CurrentContext{UserID: 82, DataScope: 1}}, riskReviewPrincipalStub{audience: &principal.Audience{UserID: 82, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}}, riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionTargetQuoteEdit): true}})
	if err != nil {
		t.Fatal(err)
	}
	f.service.targetQuoteAuthorizer = other
	if _, err = f.service.FreezeStaffTargetQuoteItemConfiguration(f.ctx, "customer-commerce-service-1", f.entitlementNo, freeze); err == nil {
		t.Fatal("another authorized staff froze this target")
	}
	f.service.targetQuoteAuthorizer = original
	// The same customer's ordinary new-purchase quote must not enter this freezer.
	ordinary := freeze
	ordinary.QuoteItemNo = f.quote.Items[0].QuoteItemNo
	if _, err = f.service.FreezeStaffTargetQuoteItemConfiguration(f.ctx, "customer-commerce-service-1", f.entitlementNo, ordinary); err == nil {
		t.Fatal("ordinary new-purchase quote entered staff target freezer")
	}
	frozen, err := f.service.FreezeStaffTargetQuoteItemConfiguration(f.ctx, "customer-commerce-service-1", f.entitlementNo, freeze)
	if err != nil || frozen == nil || !frozen.ConfigurationFrozen {
		t.Fatalf("staff freeze: %+v / %+v", frozen, err)
	}
	count, err := f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_commerce_quote_snapshot q JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc USING(quote_no) WHERE q.quote_no=? AND q.source_code='staff' AND q.purpose_code='change' AND q.created_by=81 AND q.customer_no='customer-commerce-service-1' AND sc.uploader_user_id=81 AND sc.customer_no=q.customer_no`, quote.QuoteNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("staff owner/actor evidence=%v / %+v", count, err)
	}
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_commerce_order", 1)
}
