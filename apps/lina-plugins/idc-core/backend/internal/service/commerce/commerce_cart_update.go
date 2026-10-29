// This file updates customer cart intents under the same cart and item locks as
// removal. Successful changes invalidate the old quote within the transaction.

package commerce

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// UpdateAccountItem keeps immutable configuration evidence while replacing the
// owned current intent. A repeated same-value update at current versions is a no-op.
func (s *serviceImpl) UpdateAccountItem(ctx context.Context, input UpdateItemInput) (*Cart, error) {
	if input.CartItemNo == "" || input.ExpectedCartVersion < 1 || input.ExpectedItemVersion < 1 || input.Quantity < 1 || input.Quantity > 1000 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	var (
		resolved   *catalog.PurchaseConfiguration
		snapshotNo string
	)
	if input.Purchase != nil {
		resolved, err = s.catalog.ResolvePurchaseConfiguration(ctx, *input.Purchase)
		if err != nil {
			return nil, err
		}
		if resolved == nil || input.Purchase.PurchaseCycles < resolved.MinimumPurchaseCycles {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		snapshotNo, err = newOpaqueNumber(s.random, "CFG_")
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
	}
	var cartNo string
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		cart, lockErr := lockOwnedCart(ctx, s.database, cartOwner{customerNo: customerNo})
		if lockErr != nil {
			return lockErr
		}
		if cart == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		cartNo = cart.CartNo
		if cart.Version != input.ExpectedCartVersion {
			return bizerr.NewCode(CodeConflict)
		}
		var item *entity.CommerceCartItem
		if lockErr = s.database.GetScan(ctx, &item, lockCartItemSQL, cart.CartNo, input.CartItemNo); lockErr != nil {
			return lockErr
		}
		if item == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if item.Version != input.ExpectedItemVersion {
			return bizerr.NewCode(CodeConflict)
		}
		var quantity struct {
			Quantity int `orm:"quantity"`
		}
		if lockErr = s.database.GetScan(ctx, &quantity, `SELECT quantity FROM plugin_idc_core_commerce_cart_item_quantity WHERE tenant_id=0 AND cart_item_no=? FOR UPDATE`, item.CartItemNo); lockErr != nil {
			return lockErr
		}
		if quantity.Quantity < 1 {
			return bizerr.NewCode(CodeUnavailable)
		}
		if lockErr = s.updateServerLoginChoice(ctx, customerNo, item, input, resolved); lockErr != nil {
			return lockErr
		}
		if resolved == nil && quantity.Quantity == input.Quantity {
			return nil
		}
		update := do.CommerceCartItem{Version: item.Version + 1}
		if resolved != nil {
			if cart.CurrencyCode != resolved.CurrencyCode {
				return bizerr.NewCode(CodeCurrencyConflict)
			}
			if lockErr = insertGenericPurchaseSnapshot(ctx, s.database, resolved, snapshotNo); lockErr != nil {
				return lockErr
			}
			update.ProductNo, update.VersionNo, update.BillingRuleNo = resolved.ProductNo, resolved.VersionNo, resolved.BillingRuleNo
			update.ConfigurationSnapshotNo, update.PurchaseCycles = snapshotNo, input.Purchase.PurchaseCycles
			update.StatusCode, update.IssueCode = CartItemStatusActive, ""
		}
		if _, lockErr = s.database.Model(dao.CommerceCartItem.Table()).Ctx(ctx).
			Where(do.CommerceCartItem{CartNo: cart.CartNo, CartItemNo: item.CartItemNo, TenantId: 0, Version: input.ExpectedItemVersion}).Data(update).Update(); lockErr != nil {
			return lockErr
		}
		if _, lockErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_commerce_cart_item_quantity SET quantity=? WHERE tenant_id=0 AND cart_item_no=?`, input.Quantity, item.CartItemNo); lockErr != nil {
			return lockErr
		}
		result, lockErr := s.database.Exec(ctx, updateCartVersionSQL, cart.CartNo, input.ExpectedCartVersion)
		if lockErr != nil {
			return lockErr
		}
		affected, lockErr := result.RowsAffected()
		if lockErr != nil {
			return lockErr
		}
		if affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		return nil, cartUpdateError(err)
	}
	return s.projectAccountCartByNumber(ctx, customerNo, cartNo)
}

// cartUpdateError preserves actionable mutation failures without exposing SQL.
func cartUpdateError(err error) error {
	if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeCurrencyConflict) {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}
