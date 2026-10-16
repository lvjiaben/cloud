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
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) CreateGuestCart(ctx context.Context, currency string) (*GuestCartAccess, error) {
	normalized, err := normalizeCurrency(currency)
	if err != nil {
		return nil, err
	}
	for range 4 {
		rawToken, digest, generateErr := newGuestToken(s.random)
		if generateErr != nil {
			return nil, bizerr.WrapCode(generateErr, CodeUnavailable)
		}
		cartNo, generateErr := newOpaqueNumber(s.random, "CRT_")
		if generateErr != nil {
			return nil, bizerr.WrapCode(generateErr, CodeUnavailable)
		}
		result, insertErr := s.database.Exec(ctx, createGuestCartSQL, cartNo, digest, normalized)
		if insertErr != nil {
			return nil, bizerr.WrapCode(insertErr, CodeUnavailable)
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return nil, bizerr.WrapCode(affectedErr, CodeUnavailable)
		}
		if affected == 1 {
			cart, loadErr := s.projectGuestCart(ctx, digest)
			if loadErr != nil {
				return nil, loadErr
			}
			return &GuestCartAccess{Token: rawToken, Cart: cart}, nil
		}
	}
	return nil, bizerr.NewCode(CodeUnavailable)
}

func (s *serviceImpl) GetGuestCart(ctx context.Context, token string) (*Cart, error) {
	digest, err := normalizeGuestToken(token)
	if err != nil {
		return nil, err
	}
	return s.projectGuestCart(ctx, digest)
}

func (s *serviceImpl) OpenAccountCart(ctx context.Context, currency string) (*Cart, error) {
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeCurrency(currency)
	if err != nil {
		return nil, err
	}
	var cartNo string
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(ctx, lockCustomerCartOwnerSQL, customerNo); lockErr != nil {
			return gerror.Wrap(lockErr, "serialize customer cart open")
		}
		if _, expireErr := s.database.Exec(ctx, expireCustomerCartSQL, customerNo); expireErr != nil {
			return gerror.Wrap(expireErr, "expire customer cart")
		}
		cart, loadErr := lockCustomerCart(ctx, s.database, customerNo)
		if loadErr != nil {
			return loadErr
		}
		if cart != nil {
			if cart.CurrencyCode != normalized {
				return bizerr.NewCode(CodeCurrencyConflict)
			}
			cartNo = cart.CartNo
			return nil
		}
		cartNo, loadErr = newOpaqueNumber(s.random, "CRT_")
		if loadErr != nil {
			return loadErr
		}
		_, loadErr = s.database.Exec(ctx, createCustomerCartSQL, cartNo, customerNo, normalized)
		return gerror.Wrap(loadErr, "create customer cart")
	})
	if err != nil {
		if bizerr.Is(err, CodeCurrencyConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.projectAccountCartByNumber(ctx, customerNo, cartNo)
}

func (s *serviceImpl) GetAccountCart(ctx context.Context) (*Cart, error) {
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	row, err := loadCustomerCart(ctx, s.database, customerNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return nil, nil
	}
	return s.projectAccountCartByNumber(ctx, customerNo, row.CartNo)
}

func (s *serviceImpl) AddGuestItem(ctx context.Context, token string, input AddItemInput) (*Cart, error) {
	digest, err := normalizeGuestToken(token)
	if err != nil {
		return nil, err
	}
	if err = s.addItem(ctx, cartOwner{guestDigest: digest}, input); err != nil {
		return nil, err
	}
	return s.projectGuestCart(ctx, digest)
}

func (s *serviceImpl) AddAccountItem(ctx context.Context, input AddItemInput) (*Cart, error) {
	customerNo, err := s.currentCustomerNo(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.addItem(ctx, cartOwner{customerNo: customerNo}, input); err != nil {
		return nil, err
	}
	row, err := loadCustomerCart(ctx, s.database, customerNo)
	if err != nil || row == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.projectAccountCartByNumber(ctx, customerNo, row.CartNo)
}

type cartOwner struct {
	customerNo  string
	guestDigest string
}

func (s *serviceImpl) addItem(ctx context.Context, owner cartOwner, input AddItemInput) error {
	if input.ExpectedCartVersion < 1 || input.Quantity < 1 || input.Quantity > 1000 {
		return bizerr.NewCode(CodeInvalid)
	}
	resolved, err := s.catalog.ResolvePurchaseConfiguration(ctx, input.Purchase)
	if err != nil {
		return err
	}
	if resolved == nil || input.Purchase.PurchaseCycles < resolved.MinimumPurchaseCycles {
		return bizerr.NewCode(CodeInvalid)
	}
	if resolved.RequiresCustomerContext && owner.customerNo == "" {
		return bizerr.NewCode(CodeNotAccessible)
	}
	numbers, err := s.newSnapshotNumbers()
	if err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		cart, lockErr := lockOwnedCart(ctx, s.database, owner)
		if lockErr != nil {
			return lockErr
		}
		if cart == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if cart.Version != input.ExpectedCartVersion {
			return bizerr.NewCode(CodeConflict)
		}
		if cart.CurrencyCode != resolved.CurrencyCode {
			return bizerr.NewCode(CodeCurrencyConflict)
		}
		capacity, capacityErr := loadCartCapacity(ctx, s.database, cart.CartNo)
		if capacityErr != nil {
			return capacityErr
		}
		if capacity.ItemCount >= maximumCartItems {
			return bizerr.NewCode(CodeCapacityExceeded)
		}
		if insertErr := insertGenericPurchaseSnapshot(ctx, s.database, resolved, numbers.snapshotNo); insertErr != nil {
			return insertErr
		}
		if _, insertErr := s.database.Model(dao.CommerceCartItem.Table()).Ctx(ctx).Data(do.CommerceCartItem{
			TenantId: idccap.DefaultTenantID, CartItemNo: numbers.cartItemNo,
			CartNo: cart.CartNo, ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo,
			BillingRuleNo:           resolved.BillingRuleNo,
			ConfigurationSnapshotNo: numbers.snapshotNo,
			PurchaseCycles:          input.Purchase.PurchaseCycles,
			StatusCode:              CartItemStatusActive, IssueCode: "", SortOrder: capacity.MaximumSortOrder + 1,
			Version: 1,
		}).Insert(); insertErr != nil {
			return gerror.Wrap(insertErr, "insert cart item")
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_cart_item_quantity
(tenant_id,cart_item_no,quantity) VALUES(0,?,?)`, numbers.cartItemNo, input.Quantity); insertErr != nil {
			return gerror.Wrap(insertErr, "insert generic cart item quantity")
		}
		result, updateErr := s.database.Exec(ctx, updateCartVersionSQL,
			cart.CartNo, input.ExpectedCartVersion,
		)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "advance cart version")
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeNotAccessible) || bizerr.Is(err, CodeConflict) ||
			bizerr.Is(err, CodeCurrencyConflict) || bizerr.Is(err, CodeCapacityExceeded) {
			return err
		}
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return nil
}

type snapshotNumbers struct {
	snapshotNo string
	cartItemNo string
}

func (s *serviceImpl) newSnapshotNumbers() (snapshotNumbers, error) {
	snapshotNo, err := newOpaqueNumber(s.random, "CFG_")
	if err != nil {
		return snapshotNumbers{}, err
	}
	cartItemNo, err := newOpaqueNumber(s.random, "CIT_")
	if err != nil {
		return snapshotNumbers{}, err
	}
	return snapshotNumbers{snapshotNo: snapshotNo, cartItemNo: cartItemNo}, nil
}

func insertGenericPurchaseSnapshot(
	ctx context.Context,
	database gdb.DB,
	resolved *catalog.PurchaseConfiguration,
	snapshotNo string,
) error {
	if len(resolved.PriceSelectionDigest) != 64 || len(resolved.CustomerValueDigest) != 64 || len(resolved.PriceSelections) > 128 {
		return gerror.New("generic configuration price selection is invalid")
	}
	totalSelected := 0
	for _, selection := range resolved.PriceSelections {
		if selection.DimensionNo == "" || selection.Quantity < 1 || int64(selection.Quantity) > 2147483647 {
			return gerror.New("generic configuration price selection is invalid")
		}
		totalSelected += selection.Quantity
	}
	contentDigest, err := quoteContentDigest(struct {
		PriceSelectionDigest string
		CustomerValueDigest  string
	}{resolved.PriceSelectionDigest, resolved.CustomerValueDigest})
	if err != nil {
		return gerror.Wrap(err, "digest generic purchase configuration")
	}
	if _, err := database.Model(dao.CommerceConfigurationSnapshot.Table()).Ctx(ctx).Data(struct {
		TenantId                int    `orm:"tenant_id"`
		ConfigurationSnapshotNo string `orm:"configuration_snapshot_no"`
		ProductNo               string `orm:"product_no"`
		VersionNo               string `orm:"version_no"`
		SourceRevisionDigest    string `orm:"source_revision_digest"`
		SelectionDigest         string `orm:"selection_digest"`
		CustomerValueDigest     string `orm:"customer_value_digest"`
		SelectedOptionCount     int    `orm:"selected_option_count"`
		TotalSelectedQuantity   int    `orm:"total_selected_quantity"`
		ContentDigest           string `orm:"content_digest"`
	}{
		TenantId: idccap.DefaultTenantID, ConfigurationSnapshotNo: snapshotNo,
		ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo,
		SourceRevisionDigest: resolved.SourceRevisionDigest,
		SelectionDigest:      resolved.PriceSelectionDigest, CustomerValueDigest: resolved.CustomerValueDigest,
		SelectedOptionCount: len(resolved.PriceSelections), TotalSelectedQuantity: totalSelected, ContentDigest: contentDigest,
	}).Insert(); err != nil {
		return gerror.Wrap(err, "insert generic purchase snapshot")
	}
	if len(resolved.PriceSelections) > 0 {
		rows := make([]struct {
			TenantID                int    `orm:"tenant_id"`
			ConfigurationSnapshotNo string `orm:"configuration_snapshot_no"`
			VersionNo               string `orm:"version_no"`
			DimensionNo             string `orm:"dimension_no"`
			Quantity                int    `orm:"quantity"`
		}, len(resolved.PriceSelections))
		for index, selection := range resolved.PriceSelections {
			rows[index].TenantID = idccap.DefaultTenantID
			rows[index].ConfigurationSnapshotNo = snapshotNo
			rows[index].VersionNo = resolved.VersionNo
			rows[index].DimensionNo = selection.DimensionNo
			rows[index].Quantity = selection.Quantity
		}
		if _, err := database.Model("plugin_idc_core_commerce_configuration_price_selection").Ctx(ctx).Data(rows).Insert(); err != nil {
			return gerror.Wrap(err, "insert generic configuration price selections")
		}
	}
	return nil
}

type cartCapacity struct {
	ItemCount        int `orm:"item_count"`
	MaximumSortOrder int `orm:"maximum_sort_order"`
}

func loadCartCapacity(ctx context.Context, database gdb.DB, cartNo string) (cartCapacity, error) {
	var result cartCapacity
	if err := database.GetScan(ctx, &result, cartCapacitySQL, cartNo); err != nil {
		return cartCapacity{}, gerror.Wrap(err, "load cart capacity")
	}
	return result, nil
}

const createGuestCartSQL = `
INSERT INTO plugin_idc_core_commerce_cart (
    tenant_id, cart_no, guest_owner_digest, currency_code, status_code, expires_at
) VALUES (0, ?, ?, ?, 'active', CURRENT_TIMESTAMP + INTERVAL '30 days')
ON CONFLICT DO NOTHING`

const createCustomerCartSQL = `
INSERT INTO plugin_idc_core_commerce_cart (
    tenant_id, cart_no, customer_no, currency_code, status_code, expires_at
) VALUES (0, ?, ?, ?, 'active', CURRENT_TIMESTAMP + INTERVAL '30 days')`

const expireCustomerCartSQL = `
UPDATE plugin_idc_core_commerce_cart
SET status_code = 'expired', closed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active'
  AND expires_at <= CURRENT_TIMESTAMP`

const updateCartVersionSQL = `
UPDATE plugin_idc_core_commerce_cart
SET latest_quote_no = NULL, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND cart_no = ? AND status_code = 'active' AND version = ?`

const cartCapacitySQL = `
SELECT COUNT(cart_item.id)::INT AS item_count,
       COALESCE(MAX(cart_item.sort_order), 0)::INT AS maximum_sort_order
FROM plugin_idc_core_commerce_cart_item cart_item
WHERE cart_item.tenant_id = 0 AND cart_item.cart_no = ?
  AND cart_item.status_code <> 'removed'`

func lockOwnedCart(ctx context.Context, database gdb.DB, owner cartOwner) (*entity.CommerceCart, error) {
	if (owner.customerNo == "") == (owner.guestDigest == "") {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var row *entity.CommerceCart
	var err error
	if owner.customerNo != "" {
		err = database.GetScan(ctx, &row, lockCustomerCartSQL, owner.customerNo)
	} else {
		err = database.GetScan(ctx, &row, lockGuestCartSQL, owner.guestDigest)
	}
	if err != nil {
		return nil, gerror.Wrap(err, "lock owned cart")
	}
	return row, nil
}

func lockCustomerCart(ctx context.Context, database gdb.DB, customerNo string) (*entity.CommerceCart, error) {
	return lockOwnedCart(ctx, database, cartOwner{customerNo: customerNo})
}

const lockCustomerCartSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active'
  AND expires_at > CURRENT_TIMESTAMP
LIMIT 1 FOR UPDATE`

const lockGuestCartSQL = `
SELECT id, tenant_id, cart_no, customer_no, guest_owner_digest, currency_code,
       status_code, latest_quote_no, merged_into_cart_no, expires_at, closed_at,
       version, created_at, updated_at
FROM plugin_idc_core_commerce_cart
WHERE tenant_id = 0 AND guest_owner_digest = ? AND status_code = 'active'
  AND expires_at > CURRENT_TIMESTAMP
LIMIT 1 FOR UPDATE`
