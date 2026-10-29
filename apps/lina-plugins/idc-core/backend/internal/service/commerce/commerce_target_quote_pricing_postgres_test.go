// Configuration changes retain real negotiated prices while purchase marketing
// remains outside the change purpose. All prices and snapshots use owner DDL.
package commerce

import (
	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTargetQuoteRetainsCustomerPriceWithoutPurchaseReservationPostgreSQL(t *testing.T) {
	f := newTargetQuoteFixture(t)
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate production pricing migrations")
	}
	for _, name := range []string{"072-sales-channel-foundation.sql", "116-sales-management-pricing.sql", "124-agent-level-default-price.sql"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "manifest", "sql", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range dialect.SplitSQLStatements(string(data)) {
			if _, err = f.database.Exec(f.ctx, statement); err != nil {
				t.Fatalf("%s: %+v", name, err)
			}
		}
	}
	_, err := f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_sales_purchase_price(price_no,customer_no,product_no,currency_code,charge_type_code,period_unit_code,period_count,adjustment_type_code,fixed_price_minor,version,content_digest) SELECT 'PRC_owned_target',e.customer_no,e.product_no,e.currency_code,'recurring',e.period_unit_code,e.period_count,'fixed_price',8000,1,repeat('a',64) FROM plugin_idc_core_service_entitlement e WHERE entitlement_no=?`, f.entitlementNo)
	if err != nil {
		t.Fatal(err)
	}
	prices, err := salesmanagement.NewPricing(f.database)
	if err != nil {
		t.Fatal(err)
	}
	marketingOwner, err := marketing.New(f.database, marketing.Config{})
	if err != nil {
		t.Fatal(err)
	}
	marketingEvaluator, err := NewMarketingQuoteAdjustmentEvaluator(marketingOwner)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewAgentQuoteAdjustmentEvaluator(prices, marketingEvaluator)
	if err != nil {
		t.Fatal(err)
	}
	f.service.adjustments = evaluator
	quote, err := f.service.QuoteAccountTarget(f.ctx, AccountTargetQuoteInput{EntitlementNo: f.entitlementNo, Purchase: f.purchase})
	if err != nil {
		t.Fatalf("change negotiated quote: %+v", err)
	}
	if quote.NetAmountMinor != 8000 || quote.TaxAmountMinor != 480 || quote.TotalAmountMinor != 8480 {
		t.Fatalf("negotiated target price/tax was lost: %+v", quote)
	}
	count, err := f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_commerce_quote_adjustment_snapshot a JOIN plugin_idc_core_commerce_quote_item_snapshot i USING(quote_item_no) WHERE i.quote_no=? AND a.adjustment_type_code='agent_price' AND original_period_amount_minor=10000 AND target_period_amount_minor=8000`, quote.QuoteNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("real negotiated source not frozen: %v / %+v", count, err)
	}
	count, err = f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation WHERE quote_reference=?`, quote.QuoteNo)
	if err != nil || count.Int() != 0 {
		t.Fatalf("change reserved new-purchase marketing: %v / %+v", count, err)
	}
}
