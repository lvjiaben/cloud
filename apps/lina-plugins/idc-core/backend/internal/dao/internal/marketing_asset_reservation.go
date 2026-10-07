// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingAssetReservationDao is the data access object for the table plugin_idc_core_marketing_asset_reservation.
type MarketingAssetReservationDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  MarketingAssetReservationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// MarketingAssetReservationColumns defines and stores column names for the table plugin_idc_core_marketing_asset_reservation.
type MarketingAssetReservationColumns struct {
	Id                  string //
	TenantId            string //
	ReservationNo       string //
	AssetNo             string //
	CustomerNo          string //
	QuoteReference      string //
	CurrencyCode        string //
	ReservedUses        string //
	DiscountAmountMinor string //
	StatusCode          string //
	ExpiresAt           string //
	ConsumedAt          string //
	ReleasedAt          string //
	CreatedAt           string //
	UpdatedAt           string //
	ReversedAt          string //
}

// marketingAssetReservationColumns holds the columns for the table plugin_idc_core_marketing_asset_reservation.
var marketingAssetReservationColumns = MarketingAssetReservationColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	ReservationNo:       "reservation_no",
	AssetNo:             "asset_no",
	CustomerNo:          "customer_no",
	QuoteReference:      "quote_reference",
	CurrencyCode:        "currency_code",
	ReservedUses:        "reserved_uses",
	DiscountAmountMinor: "discount_amount_minor",
	StatusCode:          "status_code",
	ExpiresAt:           "expires_at",
	ConsumedAt:          "consumed_at",
	ReleasedAt:          "released_at",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
	ReversedAt:          "reversed_at",
}

// NewMarketingAssetReservationDao creates and returns a new DAO object for table data access.
func NewMarketingAssetReservationDao(handlers ...gdb.ModelHandler) *MarketingAssetReservationDao {
	return &MarketingAssetReservationDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_asset_reservation",
		columns:  marketingAssetReservationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingAssetReservationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingAssetReservationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingAssetReservationDao) Columns() MarketingAssetReservationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingAssetReservationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingAssetReservationDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *MarketingAssetReservationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
