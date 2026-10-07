// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceCartItemDao is the data access object for the table plugin_idc_core_commerce_cart_item.
type CommerceCartItemDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  CommerceCartItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// CommerceCartItemColumns defines and stores column names for the table plugin_idc_core_commerce_cart_item.
type CommerceCartItemColumns struct {
	Id                      string //
	TenantId                string //
	CartItemNo              string //
	CartNo                  string //
	ProductNo               string //
	VersionNo               string //
	BillingRuleNo           string //
	ConfigurationSnapshotNo string //
	PurchaseCycles          string //
	StatusCode              string //
	IssueCode               string //
	SortOrder               string //
	Version                 string //
	RemovedAt               string //
	CreatedAt               string //
	UpdatedAt               string //
}

// commerceCartItemColumns holds the columns for the table plugin_idc_core_commerce_cart_item.
var commerceCartItemColumns = CommerceCartItemColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	CartItemNo:              "cart_item_no",
	CartNo:                  "cart_no",
	ProductNo:               "product_no",
	VersionNo:               "version_no",
	BillingRuleNo:           "billing_rule_no",
	ConfigurationSnapshotNo: "configuration_snapshot_no",
	PurchaseCycles:          "purchase_cycles",
	StatusCode:              "status_code",
	IssueCode:               "issue_code",
	SortOrder:               "sort_order",
	Version:                 "version",
	RemovedAt:               "removed_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewCommerceCartItemDao creates and returns a new DAO object for table data access.
func NewCommerceCartItemDao(handlers ...gdb.ModelHandler) *CommerceCartItemDao {
	return &CommerceCartItemDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_cart_item",
		columns:  commerceCartItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceCartItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceCartItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceCartItemDao) Columns() CommerceCartItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceCartItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceCartItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceCartItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
