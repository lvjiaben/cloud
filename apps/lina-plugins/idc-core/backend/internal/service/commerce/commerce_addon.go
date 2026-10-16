// This file freezes a customer-owned add-on target on the ordinary quote and
// rechecks it before checkout. Prices, taxes, periods and payments keep their
// normal commerce owners; add-ons never implicitly resize their parent.

package commerce

import (
	"context"
	"lina-core/pkg/bizerr"
	"strings"
)

// AddonService is the bounded customer directory for active target services.
// Ownership comes from the active principal; callers cannot select a customer.
type AddonService interface {
	// ListAddonTargets returns at most 100 active services and their allowed offers.
	ListAddonTargets(context.Context) ([]AddonTarget, error)
}

// AddonTarget contains public names and slugs only, never provider identifiers.
type AddonTarget struct {
	EntitlementNo   string
	ProductNameZhCN string
	ProductNameEnUS string
	Addons          []AddonOffer
}

// AddonOffer is a product allowed for one target, priced by normal commerce.
type AddonOffer struct{ PublicSlug, NameZhCN, NameEnUS string }

func (s *serviceImpl) ListAddonTargets(ctx context.Context) ([]AddonTarget, error) {
	customer, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	var targets []AddonTarget
	err = s.database.GetScan(ctx, &targets, `SELECT service.entitlement_no,COALESCE(item.product_name_zh_cn,product.name_zh_cn) AS product_name_zh_cn,COALESCE(item.product_name_en_us,product.name_en_us) AS product_name_en_us
 FROM plugin_idc_core_service_entitlement service
 JOIN plugin_idc_core_catalog_product product ON product.tenant_id=service.tenant_id AND product.product_no=service.product_no
 LEFT JOIN plugin_idc_core_commerce_order_item item ON item.tenant_id=service.tenant_id AND item.order_item_no=service.order_item_no
 WHERE service.tenant_id=0 AND service.customer_no=? AND service.status_code='active' AND (service.expires_at IS NULL OR service.expires_at>CURRENT_TIMESTAMP)
 AND EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_addon link JOIN plugin_idc_core_catalog_product offer ON offer.tenant_id=link.tenant_id AND offer.product_no=link.addon_product_no WHERE link.tenant_id=service.tenant_id AND link.product_no=service.product_no AND offer.status_code='active' AND offer.deleted_at IS NULL)
 ORDER BY service.entitlement_no LIMIT 100`, customer)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(targets) == 0 {
		return []AddonTarget{}, nil
	}
	nos := make([]string, len(targets))
	for i := range targets {
		nos[i] = targets[i].EntitlementNo
		targets[i].Addons = []AddonOffer{}
	}
	var offers []struct{ EntitlementNo, PublicSlug, NameZhCN, NameEnUS string }
	err = s.database.GetScan(ctx, &offers, `SELECT service.entitlement_no,offer.public_slug,offer.name_zh_cn,offer.name_en_us FROM plugin_idc_core_service_entitlement service JOIN plugin_idc_core_catalog_product_addon link ON link.tenant_id=service.tenant_id AND link.product_no=service.product_no JOIN plugin_idc_core_catalog_product offer ON offer.tenant_id=link.tenant_id AND offer.product_no=link.addon_product_no WHERE service.tenant_id=0 AND service.customer_no=? AND service.entitlement_no IN(?) AND offer.status_code='active' AND offer.deleted_at IS NULL ORDER BY service.entitlement_no,offer.sort_order,offer.product_no LIMIT 10001`, customer, nos)
	if err != nil || len(offers) > 10000 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	indexes := make(map[string]int, len(targets))
	for i := range targets {
		indexes[targets[i].EntitlementNo] = i
	}
	for _, o := range offers {
		i := indexes[o.EntitlementNo]
		targets[i].Addons = append(targets[i].Addons, AddonOffer{PublicSlug: o.PublicSlug, NameZhCN: o.NameZhCN, NameEnUS: o.NameEnUS})
	}
	return targets, nil
}

func (s *serviceImpl) freezeAddonQuoteTarget(ctx context.Context, customer, quoteNo, target string) error {
	if target == "" {
		return nil
	}
	if strings.TrimSpace(target) != target || len(target) > 128 || customer == "" {
		return bizerr.NewCode(CodeInvalid)
	}
	if err := s.validateAddonQuoteTarget(ctx, customer, quoteNo, target); err != nil {
		return err
	}
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_addon_target(tenant_id,quote_no,customer_no,entitlement_no,product_no) SELECT 0,?,?,entitlement_no,product_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND customer_no=? AND entitlement_no=?`, quoteNo, customer, customer, target)
	return err
}

func (s *serviceImpl) validateAddonCheckout(ctx context.Context, customer, quoteNo string) error {
	value, err := s.database.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_commerce_addon_target WHERE tenant_id=0 AND customer_no=? AND quote_no=?`, customer, quoteNo)
	if err != nil {
		return err
	}
	if value.IsEmpty() {
		return nil
	}
	return s.validateAddonQuoteTarget(ctx, customer, quoteNo, value.String())
}

func (s *serviceImpl) validateAddonQuoteTarget(ctx context.Context, customer, quoteNo, target string) error {
	var row *struct{ ProductNo string }
	err := s.database.GetScan(ctx, &row, `SELECT product_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND customer_no=? AND entitlement_no=? AND status_code='active' AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP) FOR SHARE`, customer, target)
	if err != nil {
		return err
	}
	if row == nil {
		return bizerr.NewCode(CodeNotAccessible)
	}
	count, err := s.database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_commerce_quote_item_snapshot item WHERE item.tenant_id=0 AND item.quote_no=? AND (item.charge_type_code='trial' OR NOT EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_addon link WHERE link.tenant_id=item.tenant_id AND link.product_no=? AND link.addon_product_no=item.product_no))`, quoteNo, row.ProductNo)
	if err != nil {
		return err
	}
	if count.Int() != 0 {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return nil
}
