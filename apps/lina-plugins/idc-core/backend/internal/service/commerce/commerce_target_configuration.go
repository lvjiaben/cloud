// Staff freezing is limited to its own detached change quote and exact entitlement.
package commerce

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
)

// FreezeStaffTargetQuoteItemConfiguration never impersonates a customer or
// accepts another employee's, another entitlement's, or ordinary purchase quote.
func (s *serviceImpl) FreezeStaffTargetQuoteItemConfiguration(ctx context.Context, customerNo, entitlementNo string, input FreezeQuoteItemConfigurationInput) (*FrozenQuoteItemConfiguration, error) {
	if s.targetQuoteAuthorizer == nil || !validAssistedIdentifier(customerNo) || !validAssistedIdentifier(entitlementNo) {
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
	var frozen *FrozenQuoteItemConfiguration
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, err := s.lockTargetQuoteEntitlement(ctx, customerNo, entitlementNo)
		if err != nil {
			return err
		}
		var row *struct{ InputDigest, ConfigurationSnapshotNo, ProductNo, VersionNo string }
		err = s.database.GetScan(ctx, &row, `SELECT q.input_digest,i.configuration_snapshot_no,i.product_no,i.version_no
   FROM plugin_idc_core_commerce_quote_snapshot q JOIN plugin_idc_core_commerce_quote_item_snapshot i ON i.tenant_id=q.tenant_id AND i.quote_no=q.quote_no
   WHERE q.tenant_id=0 AND q.customer_no=? AND q.created_by=? AND q.source_code='staff' AND q.purpose_code='change' AND q.item_count=1
   AND i.quote_item_no=? AND EXISTS(SELECT 1 FROM plugin_idc_core_commerce_quote_item_quantity_snapshot quantity WHERE quantity.tenant_id=i.tenant_id AND quantity.quote_item_no=i.quote_item_no AND quantity.quantity=1) AND q.expires_at>CURRENT_TIMESTAMP`, customerNo, actor, input.QuoteItemNo)
		if err != nil {
			return err
		}
		if row == nil || row.ProductNo != target.ProductNo || row.VersionNo != target.VersionNo {
			return bizerr.NewCode(CodeNotAccessible)
		}
		digest, err := quoteContentDigest(struct{ EntitlementNo, CustomerNo, ConfigurationNo string }{entitlementNo, customerNo, row.ConfigurationSnapshotNo})
		if err != nil {
			return err
		}
		if !constantTimeDigestEqual(digest, row.InputDigest) {
			return bizerr.NewCode(CodeNotAccessible)
		}
		frozen, err = s.freezeQuoteItemConfiguration(ctx, customerNo, actor, input)
		return err
	})
	if err != nil {
		return nil, mapTargetQuoteError(err)
	}
	return frozen, nil
}
