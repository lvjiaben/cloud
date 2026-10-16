// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteAdjustmentSnapshotDao is the data access object for the table plugin_idc_core_commerce_quote_adjustment_snapshot.
type CommerceQuoteAdjustmentSnapshotDao struct {
	table    string                                 // table is the underlying table name of the DAO.
	group    string                                 // group is the database configuration group name of the current DAO.
	columns  CommerceQuoteAdjustmentSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                     // handlers for customized model modification.
}

// CommerceQuoteAdjustmentSnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_quote_adjustment_snapshot.
type CommerceQuoteAdjustmentSnapshotColumns struct {
	Id                        string //
	TenantId                  string //
	QuoteAdjustmentNo         string //
	QuoteItemNo               string //
	CurrencyCode              string //
	ApplicationOrder          string //
	AdjustmentTypeCode        string //
	SourceReference           string //
	SourceVersionDigest       string //
	ReservationReference      string //
	AmountMinor               string //
	ContentDigest             string //
	CreatedAt                 string //
	OriginalPeriodAmountMinor string //
	TargetPeriodAmountMinor   string //
}

// commerceQuoteAdjustmentSnapshotColumns holds the columns for the table plugin_idc_core_commerce_quote_adjustment_snapshot.
var commerceQuoteAdjustmentSnapshotColumns = CommerceQuoteAdjustmentSnapshotColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	QuoteAdjustmentNo:         "quote_adjustment_no",
	QuoteItemNo:               "quote_item_no",
	CurrencyCode:              "currency_code",
	ApplicationOrder:          "application_order",
	AdjustmentTypeCode:        "adjustment_type_code",
	SourceReference:           "source_reference",
	SourceVersionDigest:       "source_version_digest",
	ReservationReference:      "reservation_reference",
	AmountMinor:               "amount_minor",
	ContentDigest:             "content_digest",
	CreatedAt:                 "created_at",
	OriginalPeriodAmountMinor: "original_period_amount_minor",
	TargetPeriodAmountMinor:   "target_period_amount_minor",
}

// NewCommerceQuoteAdjustmentSnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceQuoteAdjustmentSnapshotDao(handlers ...gdb.ModelHandler) *CommerceQuoteAdjustmentSnapshotDao {
	return &CommerceQuoteAdjustmentSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_quote_adjustment_snapshot",
		columns:  commerceQuoteAdjustmentSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceQuoteAdjustmentSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceQuoteAdjustmentSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceQuoteAdjustmentSnapshotDao) Columns() CommerceQuoteAdjustmentSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceQuoteAdjustmentSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceQuoteAdjustmentSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceQuoteAdjustmentSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
