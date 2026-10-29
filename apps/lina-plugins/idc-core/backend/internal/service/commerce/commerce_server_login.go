// This file checks optional private server login references before persisting
// an independent one-server cart intent. Passwords never enter commerce values.
package commerce

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/purchaseserverlogin"
)

func (s *serviceImpl) resolveServerLogin(ctx context.Context, token, customerNo, versionNo string, quantity int) (*purchaseserverlogin.Resolved, error) {
	if token == "" {
		return nil, nil
	}
	if quantity != 1 || customerNo == "" || s.serverLogins == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return s.serverLogins.Resolve(ctx, token, customerNo, versionNo)
}

func (s *serviceImpl) bindServerLogin(ctx context.Context, token, customer, version, item string) error {
	if token == "" {
		return nil
	}
	if s.serverLogins == nil {
		return bizerr.NewCode(CodeInvalid)
	}
	return s.serverLogins.Bind(ctx, token, customer, version, item)
}

func (s *serviceImpl) reusedServerLoginItem(ctx context.Context, token, customer, version, cart string, quantity int) (bool, error) {
	choice, err := s.resolveServerLogin(ctx, token, customer, version, quantity)
	if err != nil {
		return false, err
	}
	if choice == nil || choice.CartItemNo == "" {
		return false, nil
	}
	var existing struct{ CartItemNo string }
	if err = s.database.GetScan(ctx, &existing, `SELECT cart_item_no FROM plugin_idc_core_commerce_cart_item WHERE tenant_id=0 AND cart_no=? AND cart_item_no=? AND status_code='active'`, cart, choice.CartItemNo); err != nil {
		return false, err
	}
	if existing.CartItemNo == "" {
		return false, bizerr.NewCode(CodeConflict)
	}
	return true, nil
}

func (s *serviceImpl) updateServerLoginChoice(ctx context.Context, customer string, item *entity.CommerceCartItem, input UpdateItemInput, resolved *catalog.PurchaseConfiguration) error {
	var current struct{ ServerLoginToken string }
	if err := s.database.GetScan(ctx, &current, `SELECT server_login_token FROM plugin_idc_core_commerce_configuration_snapshot WHERE tenant_id=0 AND configuration_snapshot_no=?`, item.ConfigurationSnapshotNo); err != nil {
		return err
	}
	token, version := current.ServerLoginToken, item.VersionNo
	if resolved != nil {
		version = resolved.VersionNo
		if input.Purchase.ServerLoginToken != "" {
			token = input.Purchase.ServerLoginToken
		}
	}
	if _, err := s.resolveServerLogin(ctx, token, customer, version, input.Quantity); err != nil {
		return err
	}
	if resolved == nil {
		return nil
	}
	resolved.ServerLoginToken = token
	return s.bindServerLogin(ctx, token, customer, version, item.CartItemNo)
}

func (s *serviceImpl) quoteServerLoginChoice(ctx context.Context, customer string, item *structuredSnapshotQuoteItem, token string) (string, error) {
	if token == "" {
		token = item.ServerLoginToken
		if item.CartItemNo != "" {
			var current struct{ ServerLoginToken string }
			if err := s.database.GetScan(ctx, &current, `SELECT configuration.server_login_token FROM plugin_idc_core_commerce_cart_item item JOIN plugin_idc_core_commerce_cart cart ON cart.cart_no=item.cart_no AND cart.tenant_id=item.tenant_id JOIN plugin_idc_core_commerce_configuration_snapshot configuration ON configuration.configuration_snapshot_no=item.configuration_snapshot_no AND configuration.tenant_id=item.tenant_id WHERE item.tenant_id=0 AND item.cart_item_no=? AND cart.customer_no=? AND item.version_no=?`, item.CartItemNo, customer, item.VersionNo); err != nil {
				return "", err
			}
			if current.ServerLoginToken != "" {
				token = current.ServerLoginToken
			}
		}
	}
	if _, err := s.resolveServerLogin(ctx, token, customer, item.VersionNo, item.Quantity); err != nil {
		return "", err
	}
	return token, nil
}

// persistQuoteServerLogin keeps the current cart preference alongside immutable
// quote evidence. Only the private login token changes; priced configuration
// and cart quote version remain unchanged, so the current quote stays payable.
func (s *serviceImpl) persistQuoteServerLogin(ctx context.Context, customer string, quote *structuredSnapshotQuoteItem, token string) error {
	if token == "" {
		return nil
	}
	cart, err := lockOwnedCart(ctx, s.database, cartOwner{customerNo: customer})
	if err != nil {
		return err
	}
	if cart == nil || cart.CartNo != quote.CartNo || cart.LatestQuoteNo == nil || *cart.LatestQuoteNo != quote.QuoteNo {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	var item *entity.CommerceCartItem
	if err = s.database.GetScan(ctx, &item, lockCartItemSQL, cart.CartNo, quote.CartItemNo); err != nil {
		return err
	}
	if item == nil || item.VersionNo != quote.VersionNo || item.ProductNo != quote.ProductNo || item.StatusCode != CartItemStatusActive {
		return bizerr.NewCode(CodeConflict)
	}
	if err = s.bindServerLogin(ctx, token, customer, item.VersionNo, item.CartItemNo); err != nil {
		return err
	}
	var source *entity.CommerceConfigurationSnapshot
	if err = s.database.Model(do.CommerceConfigurationSnapshot{}).Ctx(ctx).Where(do.CommerceConfigurationSnapshot{TenantId: 0, ConfigurationSnapshotNo: item.ConfigurationSnapshotNo}).Scan(&source); err != nil {
		return err
	}
	if source == nil || source.VersionNo != quote.VersionNo || !constantTimeDigestEqual(source.SelectionDigest, quote.PriceSelectionDigest) || !constantTimeDigestEqual(source.CustomerValueDigest, quote.CustomerValueDigest) {
		return bizerr.NewCode(CodeConflict)
	}
	if source.ServerLoginToken == token {
		return nil
	}
	snapshotNo, err := newOpaqueNumber(s.random, "CFG_")
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	if err = s.copyServerLoginSnapshot(ctx, source.ConfigurationSnapshotNo, snapshotNo, token); err != nil {
		return err
	}
	result, err := s.database.Model(do.CommerceCartItem{}).Ctx(ctx).Where(do.CommerceCartItem{TenantId: 0, CartNo: cart.CartNo, CartItemNo: item.CartItemNo, Version: item.Version}).Data(do.CommerceCartItem{ConfigurationSnapshotNo: snapshotNo, Version: gdb.Raw("version + 1")}).Update()
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) copyServerLoginSnapshot(ctx context.Context, source, target, token string) error {
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_snapshot
(tenant_id,configuration_snapshot_no,product_no,version_no,source_revision_digest,selection_digest,selected_option_count,total_selected_quantity,content_digest,customer_value_digest,typed_value_snapshot,server_login_token)
SELECT tenant_id,?,product_no,version_no,source_revision_digest,selection_digest,selected_option_count,total_selected_quantity,content_digest,customer_value_digest,typed_value_snapshot,?
FROM plugin_idc_core_commerce_configuration_snapshot WHERE tenant_id=0 AND configuration_snapshot_no=?`, target, token, source)
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_price_selection
(tenant_id,configuration_snapshot_no,version_no,dimension_no,quantity)
SELECT tenant_id,?,version_no,dimension_no,quantity FROM plugin_idc_core_commerce_configuration_price_selection WHERE tenant_id=0 AND configuration_snapshot_no=?`, target, source)
	return err
}
