package commerce

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) SwitchGuestCurrency(
	ctx context.Context,
	token string,
	currency string,
	expectedVersion int64,
) (*Cart, error) {
	digest, err := normalizeGuestToken(token)
	if err != nil {
		return nil, err
	}
	if _, err = s.switchCurrency(ctx, cartOwner{guestDigest: digest}, currency, expectedVersion); err != nil {
		return nil, err
	}
	return s.projectGuestCart(ctx, digest)
}

func (s *serviceImpl) SwitchAccountCurrency(
	ctx context.Context,
	currency string,
	expectedVersion int64,
) (*Cart, error) {
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	cartNo, err := s.switchCurrency(ctx, cartOwner{customerNo: customerNo}, currency, expectedVersion)
	if err != nil {
		return nil, err
	}
	return s.projectAccountCartByNumber(ctx, customerNo, cartNo)
}

func (s *serviceImpl) switchCurrency(
	ctx context.Context,
	owner cartOwner,
	currency string,
	expectedVersion int64,
) (string, error) {
	normalized, err := normalizeCurrency(currency)
	if err != nil || expectedVersion < 1 {
		return "", bizerr.NewCode(CodeInvalid)
	}
	var targetCartNo string
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if owner.customerNo != "" {
			if _, lockErr := s.database.Exec(ctx, lockCustomerCartOwnerSQL, owner.customerNo); lockErr != nil {
				return gerror.Wrap(lockErr, "serialize customer cart currency switch")
			}
		}
		cart, lockErr := lockOwnedCart(ctx, s.database, owner)
		if lockErr != nil {
			return lockErr
		}
		if cart == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if cart.Version != expectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		if cart.CurrencyCode == normalized {
			targetCartNo = cart.CartNo
			return nil
		}
		capacity, capacityErr := loadCartCapacity(ctx, s.database, cart.CartNo)
		if capacityErr != nil {
			return capacityErr
		}
		if capacity.ItemCount != 0 {
			return bizerr.NewCode(CodeCurrencySwitchBlocked)
		}
		result, closeErr := s.database.Exec(ctx, abandonCartForCurrencySwitchSQL,
			cart.CartNo, expectedVersion,
		)
		if closeErr != nil {
			return gerror.Wrap(closeErr, "abandon cart for currency switch")
		}
		affected, closeErr := result.RowsAffected()
		if closeErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		targetCartNo, closeErr = newOpaqueNumber(s.random, "CRT_")
		if closeErr != nil {
			return closeErr
		}
		if owner.customerNo != "" {
			_, closeErr = s.database.Exec(ctx, createCustomerCartSQL,
				targetCartNo, owner.customerNo, normalized,
			)
		} else {
			_, closeErr = s.database.Exec(ctx, createGuestCartForSwitchSQL,
				targetCartNo, owner.guestDigest, normalized,
			)
		}
		return gerror.Wrap(closeErr, "create currency-switch cart")
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeNotAccessible) ||
			bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeCurrencySwitchBlocked) {
			return "", err
		}
		return "", bizerr.WrapCode(err, CodeUnavailable)
	}
	return targetCartNo, nil
}

const abandonCartForCurrencySwitchSQL = `
UPDATE plugin_idc_core_commerce_cart
SET status_code = 'abandoned', closed_at = CURRENT_TIMESTAMP,
    latest_quote_no = NULL, updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE tenant_id = 0 AND cart_no = ? AND status_code = 'active' AND version = ?`

const createGuestCartForSwitchSQL = `
INSERT INTO plugin_idc_core_commerce_cart (
    tenant_id, cart_no, guest_owner_digest, currency_code, status_code, expires_at
) VALUES (0, ?, ?, ?, 'active', CURRENT_TIMESTAMP + INTERVAL '30 days')`
