// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteItemInventorySnapshotDao is the data access object for the table plugin_idc_core_commerce_quote_item_inventory_snapshot.
type CommerceQuoteItemInventorySnapshotDao struct {
	table    string                                    // table is the underlying table name of the DAO.
	group    string                                    // group is the database configuration group name of the current DAO.
	columns  CommerceQuoteItemInventorySnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                        // handlers for customized model modification.
}

// CommerceQuoteItemInventorySnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_quote_item_inventory_snapshot.
type CommerceQuoteItemInventorySnapshotColumns struct {
	QuoteItemNo      string //
	VersionNo        string //
	TrackingEnabled  string //
	InventoryVersion string //
	SellableQuantity string //
	ContentDigest    string //
	CreatedAt        string //
}

// commerceQuoteItemInventorySnapshotColumns holds the columns for the table plugin_idc_core_commerce_quote_item_inventory_snapshot.
var commerceQuoteItemInventorySnapshotColumns = CommerceQuoteItemInventorySnapshotColumns{
	QuoteItemNo:      "quote_item_no",
	VersionNo:        "version_no",
	TrackingEnabled:  "tracking_enabled",
	InventoryVersion: "inventory_version",
	SellableQuantity: "sellable_quantity",
	ContentDigest:    "content_digest",
	CreatedAt:        "created_at",
}

// NewCommerceQuoteItemInventorySnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceQuoteItemInventorySnapshotDao(handlers ...gdb.ModelHandler) *CommerceQuoteItemInventorySnapshotDao {
	return &CommerceQuoteItemInventorySnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_quote_item_inventory_snapshot",
		columns:  commerceQuoteItemInventorySnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceQuoteItemInventorySnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceQuoteItemInventorySnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceQuoteItemInventorySnapshotDao) Columns() CommerceQuoteItemInventorySnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceQuoteItemInventorySnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceQuoteItemInventorySnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceQuoteItemInventorySnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
