// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteChargeSnapshotDao is the data access object for the table plugin_idc_core_commerce_quote_charge_snapshot.
type CommerceQuoteChargeSnapshotDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  CommerceQuoteChargeSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// CommerceQuoteChargeSnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_quote_charge_snapshot.
type CommerceQuoteChargeSnapshotColumns struct {
	Id                            string //
	TenantId                      string //
	QuoteChargeNo                 string //
	QuoteItemNo                   string //
	CurrencyCode                  string //
	LineNumber                    string //
	ChargeKindCode                string //
	ChargePhaseCode               string //
	ConfigurationSnapshotNo       string //
	OptionNo                      string //
	Quantity                      string //
	ChargeAmountMinor             string //
	SetupFeeMinor                 string //
	PricedAmountMinor             string //
	PriceSourceDigest             string //
	CreatedAt                     string //
	ConfigurationPriceDimensionNo string //
}

// commerceQuoteChargeSnapshotColumns holds the columns for the table plugin_idc_core_commerce_quote_charge_snapshot.
var commerceQuoteChargeSnapshotColumns = CommerceQuoteChargeSnapshotColumns{
	Id:                            "id",
	TenantId:                      "tenant_id",
	QuoteChargeNo:                 "quote_charge_no",
	QuoteItemNo:                   "quote_item_no",
	CurrencyCode:                  "currency_code",
	LineNumber:                    "line_number",
	ChargeKindCode:                "charge_kind_code",
	ChargePhaseCode:               "charge_phase_code",
	ConfigurationSnapshotNo:       "configuration_snapshot_no",
	OptionNo:                      "option_no",
	Quantity:                      "quantity",
	ChargeAmountMinor:             "charge_amount_minor",
	SetupFeeMinor:                 "setup_fee_minor",
	PricedAmountMinor:             "priced_amount_minor",
	PriceSourceDigest:             "price_source_digest",
	CreatedAt:                     "created_at",
	ConfigurationPriceDimensionNo: "configuration_price_dimension_no",
}

// NewCommerceQuoteChargeSnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceQuoteChargeSnapshotDao(handlers ...gdb.ModelHandler) *CommerceQuoteChargeSnapshotDao {
	return &CommerceQuoteChargeSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_quote_charge_snapshot",
		columns:  commerceQuoteChargeSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceQuoteChargeSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceQuoteChargeSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceQuoteChargeSnapshotDao) Columns() CommerceQuoteChargeSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceQuoteChargeSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceQuoteChargeSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceQuoteChargeSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
