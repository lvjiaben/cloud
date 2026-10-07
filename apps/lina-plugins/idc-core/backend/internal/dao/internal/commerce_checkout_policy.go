// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceCheckoutPolicyDao is the data access object for the table plugin_idc_core_commerce_checkout_policy.
type CommerceCheckoutPolicyDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  CommerceCheckoutPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// CommerceCheckoutPolicyColumns defines and stores column names for the table plugin_idc_core_commerce_checkout_policy.
type CommerceCheckoutPolicyColumns struct {
	Id                     string //
	TenantId               string //
	PolicyNo               string //
	PaymentValiditySeconds string //
	Version                string //
	UpdatedBy              string //
	CreatedAt              string //
	UpdatedAt              string //
}

// commerceCheckoutPolicyColumns holds the columns for the table plugin_idc_core_commerce_checkout_policy.
var commerceCheckoutPolicyColumns = CommerceCheckoutPolicyColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	PolicyNo:               "policy_no",
	PaymentValiditySeconds: "payment_validity_seconds",
	Version:                "version",
	UpdatedBy:              "updated_by",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
}

// NewCommerceCheckoutPolicyDao creates and returns a new DAO object for table data access.
func NewCommerceCheckoutPolicyDao(handlers ...gdb.ModelHandler) *CommerceCheckoutPolicyDao {
	return &CommerceCheckoutPolicyDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_checkout_policy",
		columns:  commerceCheckoutPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceCheckoutPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceCheckoutPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceCheckoutPolicyDao) Columns() CommerceCheckoutPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceCheckoutPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceCheckoutPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceCheckoutPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
