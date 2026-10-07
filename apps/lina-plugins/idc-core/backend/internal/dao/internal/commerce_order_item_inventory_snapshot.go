// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderItemInventorySnapshotDao is the data access object for the table plugin_idc_core_commerce_order_item_inventory_snapshot.
type CommerceOrderItemInventorySnapshotDao struct {
	table    string                                    // table is the underlying table name of the DAO.
	group    string                                    // group is the database configuration group name of the current DAO.
	columns  CommerceOrderItemInventorySnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                        // handlers for customized model modification.
}

// CommerceOrderItemInventorySnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_order_item_inventory_snapshot.
type CommerceOrderItemInventorySnapshotColumns struct {
	OrderItemNo      string //
	QuoteItemNo      string //
	VersionNo        string //
	TrackingEnabled  string //
	InventoryVersion string //
	SellableQuantity string //
	ContentDigest    string //
	CreatedAt        string //
}

// commerceOrderItemInventorySnapshotColumns holds the columns for the table plugin_idc_core_commerce_order_item_inventory_snapshot.
var commerceOrderItemInventorySnapshotColumns = CommerceOrderItemInventorySnapshotColumns{
	OrderItemNo:      "order_item_no",
	QuoteItemNo:      "quote_item_no",
	VersionNo:        "version_no",
	TrackingEnabled:  "tracking_enabled",
	InventoryVersion: "inventory_version",
	SellableQuantity: "sellable_quantity",
	ContentDigest:    "content_digest",
	CreatedAt:        "created_at",
}

// NewCommerceOrderItemInventorySnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceOrderItemInventorySnapshotDao(handlers ...gdb.ModelHandler) *CommerceOrderItemInventorySnapshotDao {
	return &CommerceOrderItemInventorySnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_order_item_inventory_snapshot",
		columns:  commerceOrderItemInventorySnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceOrderItemInventorySnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceOrderItemInventorySnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceOrderItemInventorySnapshotDao) Columns() CommerceOrderItemInventorySnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceOrderItemInventorySnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceOrderItemInventorySnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceOrderItemInventorySnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
