// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceCartDao is the data access object for the table plugin_idc_core_commerce_cart.
type CommerceCartDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  CommerceCartColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// CommerceCartColumns defines and stores column names for the table plugin_idc_core_commerce_cart.
type CommerceCartColumns struct {
	Id               string //
	TenantId         string //
	CartNo           string //
	CustomerNo       string //
	GuestOwnerDigest string //
	CurrencyCode     string //
	StatusCode       string //
	LatestQuoteNo    string //
	MergedIntoCartNo string //
	ExpiresAt        string //
	ClosedAt         string //
	Version          string //
	CreatedAt        string //
	UpdatedAt        string //
}

// commerceCartColumns holds the columns for the table plugin_idc_core_commerce_cart.
var commerceCartColumns = CommerceCartColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	CartNo:           "cart_no",
	CustomerNo:       "customer_no",
	GuestOwnerDigest: "guest_owner_digest",
	CurrencyCode:     "currency_code",
	StatusCode:       "status_code",
	LatestQuoteNo:    "latest_quote_no",
	MergedIntoCartNo: "merged_into_cart_no",
	ExpiresAt:        "expires_at",
	ClosedAt:         "closed_at",
	Version:          "version",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewCommerceCartDao creates and returns a new DAO object for table data access.
func NewCommerceCartDao(handlers ...gdb.ModelHandler) *CommerceCartDao {
	return &CommerceCartDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_cart",
		columns:  commerceCartColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceCartDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceCartDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceCartDao) Columns() CommerceCartColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceCartDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceCartDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceCartDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
