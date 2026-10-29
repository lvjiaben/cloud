// Owned target quotes reuse the ordinary catalog, adjustments, tax and immutable
// row builders for one existing entitlement, without reading or changing a cart.

package commerce

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// AccountTargetQuoteInput selects one published configuration for an existing
// service owned by the real current customer. Quantity is always one; money,
// product/version and the effective period are independently owner-validated.
type AccountTargetQuoteInput struct {
	EntitlementNo string
	Purchase      catalog.PurchaseConfigurationInput
}

type targetQuoteEntitlement struct {
	EntitlementNo, ProductNo, VersionNo, CurrencyCode, PeriodUnitCode string
	PeriodCount, PurchaseCycles                                       int
}

// QuoteAccountTarget creates a detached, customer-owned resize quote for a
// same-product/version/period target. It never creates or invalidates a cart.
// The caller freezes its sole QuoteItemNo with FreezeAccountQuoteItemConfiguration
// in the same outer transaction when atomic typed-configuration persistence is required.
func (s *serviceImpl) QuoteAccountTarget(ctx context.Context, input AccountTargetQuoteInput) (*Quote, error) {
	audience, err := s.currentCheckoutAudience(ctx)
	if err != nil {
		return nil, err
	}
	return s.quoteOwnedTarget(ctx, audience.CustomerNo, audience.UserID, "customer", input)
}

// QuoteStaffTarget records the real authorized employee and customer owner.
func (s *serviceImpl) QuoteStaffTarget(ctx context.Context, customerNo string, input AccountTargetQuoteInput) (*Quote, error) {
	if s.targetQuoteAuthorizer == nil || !validAssistedIdentifier(customerNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.targetQuoteAuthorizer.AuthorizeTargetQuote(ctx)
	if err != nil {
		return nil, err
	}
	for _, value := range input.Purchase.Values {
		if len(value.FileTokens) > 0 {
			return nil, bizerr.NewCode(CodeNotAccessible)
		}
	}
	return s.quoteOwnedTarget(ctx, customerNo, actor, "staff", input)
}

func (s *serviceImpl) quoteOwnedTarget(ctx context.Context, customerNo string, actor int, source string, input AccountTargetQuoteInput) (*Quote, error) {
	if input.EntitlementNo == "" || len(input.EntitlementNo) > 40 || strings.TrimSpace(input.EntitlementNo) != input.EntitlementNo || input.Purchase.PurchaseCycles < 1 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if s.quoteCatalog == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if err := s.admitCheckoutCurrency(ctx, input.Purchase.CurrencyCode); err != nil {
		return nil, err
	}
	var quote *Quote
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, loadErr := s.lockTargetQuoteEntitlement(ctx, customerNo, input.EntitlementNo)
		if loadErr != nil {
			return loadErr
		}
		resolved, loadErr := s.catalog.ResolvePurchaseConfiguration(ctx, input.Purchase)
		if loadErr != nil {
			return bizerr.WrapCode(loadErr, CodeQuoteNotAvailable)
		}
		if resolved == nil || resolved.ProductNo != target.ProductNo || resolved.VersionNo != target.VersionNo || resolved.CurrencyCode != target.CurrencyCode || input.Purchase.PurchaseCycles != target.PurchaseCycles || input.Purchase.PurchaseCycles < resolved.MinimumPurchaseCycles {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		numbers, loadErr := s.newSnapshotNumbers()
		if loadErr != nil {
			return loadErr
		}
		if loadErr = insertGenericPurchaseSnapshot(ctx, s.database, resolved, numbers.snapshotNo); loadErr != nil {
			return loadErr
		}
		now, loadErr := quoteDatabaseNow(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		expiry := now.Add(defaultQuoteLifetime)
		graph := &quoteCartGraph{cart: &entity.CommerceCart{CustomerNo: &customerNo, CurrencyCode: target.CurrencyCode, ExpiresAt: &expiry}, items: []entity.CommerceCartItem{{CartItemNo: numbers.cartItemNo, ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo, BillingRuleNo: resolved.BillingRuleNo, ConfigurationSnapshotNo: numbers.snapshotNo, PurchaseCycles: input.Purchase.PurchaseCycles}}, quantities: map[string]int{numbers.cartItemNo: 1}, priceSelections: map[string][]catalog.ConfigurationPriceSelection{numbers.snapshotNo: resolved.PriceSelections}}
		quote, loadErr = s.buildTargetQuote(ctx, graph, target, customerNo, actor, source, now)
		return loadErr
	})
	if err != nil {
		return nil, mapTargetQuoteError(err)
	}
	return quote, nil
}

// lockTargetQuoteEntitlement applies ownership in SQL before exposing any target
// facts. Customer -> parent -> product locks match native commercial admission.
func (s *serviceImpl) lockTargetQuoteEntitlement(ctx context.Context, customer, no string) (*targetQuoteEntitlement, error) {
	if _, err := s.database.GetValue(ctx, `SELECT customer_no FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? AND status_code='active' FOR UPDATE`, customer); err != nil {
		return nil, err
	}
	var row *targetQuoteEntitlement
	err := s.database.GetScan(ctx, &row, `SELECT e.entitlement_no,e.product_no,e.version_no,e.currency_code,
 COALESCE(price.period_unit_code,e.period_unit_code) AS period_unit_code,COALESCE(price.period_count,e.period_count) AS period_count,
 COALESCE(price.purchase_cycles,item.purchase_cycles) AS purchase_cycles
 FROM plugin_idc_core_service_entitlement e
 JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no AND c.status_code='active'
 JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no AND p.current_version_no=e.version_no AND p.status_code='active' AND p.deleted_at IS NULL
 JOIN plugin_idc_core_commerce_order_item item ON item.tenant_id=e.tenant_id AND item.order_item_no=e.order_item_no AND item.order_no=e.order_no
 LEFT JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=e.tenant_id AND price.price_snapshot_no=e.current_price_snapshot_no AND price.subject_type_code='entitlement' AND price.subject_no=e.entitlement_no AND price.customer_no=e.customer_no
 WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.entitlement_owner_id='idc-resource' AND e.upstream_service_no=''
 AND e.status_code='active' AND e.charge_type_code='recurring' AND e.expires_at>CURRENT_TIMESTAMP
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock l WHERE l.tenant_id=e.tenant_id AND l.entitlement_no=e.entitlement_no AND l.released_at IS NULL)
 FOR UPDATE OF e,p`, no, customer)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return row, nil
}

// buildTargetQuote delegates all pricing and row construction to the normal
// quote engine. Its detached writer preserves the actual principal provenance.
func (s *serviceImpl) buildTargetQuote(ctx context.Context, graph *quoteCartGraph, target *targetQuoteEntitlement, customerNo string, userID int, source string, now time.Time) (*Quote, error) {
	input, err := quoteCatalogInput(graph, nil)
	if err != nil {
		return nil, err
	}
	assembled, err := s.quoteCatalog.AssemblePurchaseQuote(ctx, input)
	if err != nil {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	ordered, err := orderQuoteCatalogLines(graph.items, assembled)
	if err != nil {
		return nil, err
	}
	if len(ordered) != 1 || ordered[0].ChargeTypeCode != string(catalog.ChargeTypeRecurring) || ordered[0].Quantity != 1 || ordered[0].PeriodUnitCode != target.PeriodUnitCode || ordered[0].PeriodCount != target.PeriodCount || ordered[0].PurchaseCycles != target.PurchaseCycles {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	calculated, err := s.prepareQuoteLines(ordered, graph.items)
	if err != nil {
		return nil, err
	}
	customer, err := loadQuoteCustomerFact(ctx, s.database, customerNo)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	quoteNo, err := newOpaqueNumber(s.random, "QTE_")
	if err != nil {
		return nil, err
	}
	adjustments, err := s.evaluateQuoteAdjustmentsForPurpose(ctx, quoteNo, graph.cart.CurrencyCode, now, customer, nil, calculated, quotePurposeChange)
	if err != nil {
		return nil, err
	}
	if err = applyQuoteAdjustments(calculated, adjustments); err != nil {
		return nil, err
	}
	expiry, err := quoteExpiry(now, graph.cart, calculated, adjustments)
	if err != nil {
		return nil, err
	}
	requestDigest, err := quoteContentDigest(struct {
		EntitlementNo, CustomerNo, ConfigurationNo string
	}{target.EntitlementNo, customerNo, graph.items[0].ConfigurationSnapshotNo})
	if err != nil {
		return nil, err
	}
	return s.persistAssistedQuote(ctx, quoteNo, graph, customer, now, expiry, calculated, assistedQuoteBuild{sourceCode: source, purposeCode: quotePurposeChange, createdBy: userID, requestDigest: requestDigest})
}

func mapTargetQuoteError(err error) error {
	if bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeQuoteNotAvailable) {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}
