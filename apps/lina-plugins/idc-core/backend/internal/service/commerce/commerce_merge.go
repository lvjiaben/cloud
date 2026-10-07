package commerce

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// MergeGuestCart preserves the guest cart as an idempotent merge receipt. A
// customer target is always used, even when the customer had no prior cart.
func (s *serviceImpl) MergeGuestCart(ctx context.Context, token string) (*Cart, error) {
	digest, err := normalizeGuestToken(token)
	if err != nil {
		return nil, err
	}
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	var targetCartNo string
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(ctx, lockCustomerCartOwnerSQL, customerNo); lockErr != nil {
			return gerror.Wrap(lockErr, "serialize customer cart merge")
		}
		source, lockErr := lockGuestCartForMerge(ctx, s.database, digest)
		if lockErr != nil {
			return lockErr
		}
		if source == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if source.StatusCode == "merged" {
			if source.MergedIntoCartNo == nil || *source.MergedIntoCartNo == "" {
				return bizerr.NewCode(CodeUnavailable)
			}
			target, targetErr := lockCustomerCartByNumber(ctx, s.database, customerNo, *source.MergedIntoCartNo)
			if targetErr != nil {
				return targetErr
			}
			if target == nil {
				return bizerr.NewCode(CodeNotAccessible)
			}
			targetCartNo = target.CartNo
			return nil
		}
		if source.StatusCode != "active" {
			return bizerr.NewCode(CodeNotAccessible)
		}
		target, targetErr := lockCustomerCart(ctx, s.database, customerNo)
		if targetErr != nil {
			return targetErr
		}
		if target == nil {
			targetCartNo, targetErr = newOpaqueNumber(s.random, "CRT_")
			if targetErr != nil {
				return targetErr
			}
			if _, targetErr = s.database.Exec(ctx, createCustomerCartSQL,
				targetCartNo, customerNo, source.CurrencyCode,
			); targetErr != nil {
				return gerror.Wrap(targetErr, "create merge target cart")
			}
			target, targetErr = lockCustomerCartByNumber(ctx, s.database, customerNo, targetCartNo)
			if targetErr != nil {
				return targetErr
			}
			if target == nil {
				return gerror.New("created merge target cart disappeared")
			}
		} else {
			targetCartNo = target.CartNo
		}
		if target.CurrencyCode != source.CurrencyCode {
			return bizerr.NewCode(CodeCurrencyConflict)
		}
		sourceCapacity, targetErr := loadCartCapacity(ctx, s.database, source.CartNo)
		if targetErr != nil {
			return targetErr
		}
		targetCapacity, targetErr := loadCartCapacity(ctx, s.database, target.CartNo)
		if targetErr != nil {
			return targetErr
		}
		if sourceCapacity.ItemCount+targetCapacity.ItemCount > maximumCartItems {
			return bizerr.NewCode(CodeCapacityExceeded)
		}
		if targetErr = s.copyGuestCartItems(ctx, source.CartNo, target.CartNo,
			targetCapacity.MaximumSortOrder,
		); targetErr != nil {
			return targetErr
		}
		result, targetErr := s.database.Exec(ctx, closeMergedGuestCartSQL,
			target.CartNo, source.CartNo, source.Version,
		)
		if targetErr != nil {
			return gerror.Wrap(targetErr, "close merged guest cart")
		}
		affected, targetErr := result.RowsAffected()
		if targetErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		result, targetErr = s.database.Exec(ctx, refreshMergedTargetCartSQL,
			target.CartNo, target.Version,
		)
		if targetErr != nil {
			return gerror.Wrap(targetErr, "refresh merged target cart")
		}
		affected, targetErr = result.RowsAffected()
		if targetErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeCurrencyConflict) ||
			bizerr.Is(err, CodeCapacityExceeded) || bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.projectAccountCartByNumber(ctx, customerNo, targetCartNo)
}

func (s *serviceImpl) copyGuestCartItems(
	ctx context.Context,
	sourceCartNo string,
	targetCartNo string,
	targetMaximumSortOrder int,
) error {
	rows := make([]entity.CommerceCartItem, 0)
	columns := dao.CommerceCartItem.Columns()
	if err := s.database.Model(dao.CommerceCartItem.Table()).Ctx(ctx).
		Where(do.CommerceCartItem{TenantId: idccap.DefaultTenantID, CartNo: sourceCartNo}).
		WhereNot(columns.StatusCode, "removed").
		OrderAsc(columns.SortOrder).OrderAsc(columns.Id).
		Limit(maximumCartItems + 1).Scan(&rows); err != nil {
		return gerror.Wrap(err, "load guest cart items for merge")
	}
	if len(rows) > maximumCartItems {
		return bizerr.NewCode(CodeCapacityExceeded)
	}
	if len(rows) > 0 {
		type quantityRow struct {
			CartItemNo string `orm:"cart_item_no"`
			Quantity   int    `orm:"quantity"`
		}
		quantityRows := make([]quantityRow, 0, len(rows))
		if err := s.database.GetScan(ctx, &quantityRows, `SELECT quantity.cart_item_no,quantity.quantity FROM plugin_idc_core_commerce_cart_item_quantity quantity JOIN plugin_idc_core_commerce_cart_item item ON item.tenant_id=quantity.tenant_id AND item.cart_item_no=quantity.cart_item_no WHERE item.tenant_id=0 AND item.cart_no=? AND item.status_code<>'removed' ORDER BY item.sort_order,item.id LIMIT ?`, sourceCartNo, maximumCartItems+1); err != nil || len(quantityRows) != len(rows) {
			return gerror.New("guest cart generic quantity evidence is unavailable")
		}
		quantities := make(map[string]int, len(quantityRows))
		for _, quantity := range quantityRows {
			quantities[quantity.CartItemNo] = quantity.Quantity
		}
		copies := make([]do.CommerceCartItem, 0, len(rows))
		quantityCopies := make([]map[string]any, 0, len(rows))
		for index, row := range rows {
			cartItemNo, err := newOpaqueNumber(s.random, "CIT_")
			if err != nil {
				return err
			}
			copies = append(copies, do.CommerceCartItem{
				TenantId: idccap.DefaultTenantID, CartItemNo: cartItemNo, CartNo: targetCartNo,
				ProductNo: row.ProductNo, VersionNo: row.VersionNo,
				BillingRuleNo:           row.BillingRuleNo,
				ConfigurationSnapshotNo: row.ConfigurationSnapshotNo,
				PurchaseCycles:          row.PurchaseCycles,
				StatusCode:              row.StatusCode, IssueCode: row.IssueCode,
				SortOrder: targetMaximumSortOrder + index + 1, Version: 1,
			})
			quantityCopies = append(quantityCopies, map[string]any{"tenant_id": idccap.DefaultTenantID, "cart_item_no": cartItemNo, "quantity": quantities[row.CartItemNo]})
		}
		if _, err := s.database.Model(dao.CommerceCartItem.Table()).Ctx(ctx).Data(copies).Insert(); err != nil {
			return gerror.Wrap(err, "copy guest cart items")
		}
		if _, err := s.database.Model("plugin_idc_core_commerce_cart_item_quantity").Ctx(ctx).Data(quantityCopies).Insert(); err != nil {
			return gerror.Wrap(err, "copy guest cart generic quantities")
		}
		if _, err := s.database.Exec(ctx, retireMergedGuestCartItemsSQL, sourceCartNo); err != nil {
			return gerror.Wrap(err, "retire merged guest cart items")
		}
	}
	return nil
}

func lockGuestCartForMerge(
	ctx context.Context,
	database gdb.DB,
	digest string,
) (*entity.CommerceCart, error) {
	var row *entity.CommerceCart
	if err := database.GetScan(ctx, &row, lockGuestCartForMergeSQL, digest); err != nil {
		return nil, gerror.Wrap(err, "lock guest cart for merge")
	}
	return row, nil
}

func lockCustomerCartByNumber(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	cartNo string,
) (*entity.CommerceCart, error) {
	var row *entity.CommerceCart
	if err := database.GetScan(ctx, &row, lockCustomerCartByNumberSQL, customerNo, cartNo); err != nil {
		return nil, gerror.Wrap(err, "lock customer cart by number")
	}
	return row, nil
}

const lockCustomerCartOwnerSQL = `SELECT pg_advisory_xact_lock(hashtextextended(?, 719536))`

const lockGuestCartForMergeSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND guest_owner_digest = ?
  AND (status_code = 'merged' OR (status_code = 'active' AND expires_at > CURRENT_TIMESTAMP))
LIMIT 1 FOR UPDATE`

const lockCustomerCartByNumberSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND customer_no = ? AND cart_no = ?
  AND status_code = 'active' AND expires_at > CURRENT_TIMESTAMP
LIMIT 1 FOR UPDATE`

const retireMergedGuestCartItemsSQL = `
UPDATE plugin_idc_core_commerce_cart_item
SET status_code = 'removed', issue_code = 'merged_into_customer_cart',
    removed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE tenant_id = 0 AND cart_no = ? AND status_code <> 'removed'`

const closeMergedGuestCartSQL = `
UPDATE plugin_idc_core_commerce_cart
SET status_code = 'merged', merged_into_cart_no = ?, closed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE tenant_id = 0 AND cart_no = ? AND status_code = 'active' AND version = ?`

const refreshMergedTargetCartSQL = `
UPDATE plugin_idc_core_commerce_cart
SET latest_quote_no = NULL,
    expires_at = GREATEST(expires_at, CURRENT_TIMESTAMP + INTERVAL '30 days'),
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE tenant_id = 0 AND cart_no = ? AND status_code = 'active' AND version = ?`
