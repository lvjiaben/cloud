package commerce

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) RemoveGuestItem(
	ctx context.Context,
	token string,
	cartItemNo string,
	expectedCartVersion int64,
	expectedItemVersion int64,
) (*Cart, error) {
	digest, err := normalizeGuestToken(token)
	if err != nil {
		return nil, err
	}
	if err = s.removeItem(ctx, cartOwner{guestDigest: digest}, cartItemNo, expectedCartVersion, expectedItemVersion); err != nil {
		return nil, err
	}
	return s.projectGuestCart(ctx, digest)
}

func (s *serviceImpl) RemoveAccountItem(
	ctx context.Context,
	cartItemNo string,
	expectedCartVersion int64,
	expectedItemVersion int64,
) (*Cart, error) {
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.removeItem(ctx, cartOwner{customerNo: customerNo}, cartItemNo, expectedCartVersion, expectedItemVersion); err != nil {
		return nil, err
	}
	row, err := loadCustomerCart(ctx, s.database, customerNo)
	if err != nil || row == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.projectAccountCartByNumber(ctx, customerNo, row.CartNo)
}

func (s *serviceImpl) removeItem(
	ctx context.Context,
	owner cartOwner,
	cartItemNo string,
	expectedCartVersion int64,
	expectedItemVersion int64,
) error {
	if cartItemNo == "" || expectedCartVersion < 1 || expectedItemVersion < 1 {
		return bizerr.NewCode(CodeInvalid)
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		cart, lockErr := lockOwnedCart(ctx, s.database, owner)
		if lockErr != nil {
			return lockErr
		}
		if cart == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if cart.Version != expectedCartVersion {
			return bizerr.NewCode(CodeConflict)
		}
		var item *entity.CommerceCartItem
		if lockErr = s.database.GetScan(ctx, &item, lockCartItemSQL, cart.CartNo, cartItemNo); lockErr != nil {
			return lockErr
		}
		if item == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if item.Version != expectedItemVersion {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr := s.database.Exec(ctx, removeCartItemSQL,
			item.CartItemNo, expectedItemVersion,
		)
		if updateErr != nil {
			return updateErr
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr = s.database.Exec(ctx, updateCartVersionSQL, cart.CartNo, expectedCartVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, updateErr = result.RowsAffected()
		if updateErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeConflict) {
			return err
		}
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return nil
}

const lockCartItemSQL = `
SELECT id, tenant_id, cart_item_no, cart_no, product_no, version_no,
       billing_rule_no, configuration_snapshot_no,
       purchase_cycles, status_code, issue_code, sort_order, version,
       removed_at, created_at, updated_at
FROM plugin_idc_core_commerce_cart_item
WHERE tenant_id = 0 AND cart_no = ? AND cart_item_no = ?
  AND status_code <> 'removed'
LIMIT 1 FOR UPDATE`

const removeCartItemSQL = `
UPDATE plugin_idc_core_commerce_cart_item
SET status_code = 'removed', issue_code = 'removed_by_owner',
    removed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE tenant_id = 0 AND cart_item_no = ? AND version = ?
  AND status_code <> 'removed'`
