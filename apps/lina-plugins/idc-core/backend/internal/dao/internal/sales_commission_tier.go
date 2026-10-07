// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionTierDao is the data access object for the table plugin_idc_core_sales_commission_tier.
type SalesCommissionTierDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SalesCommissionTierColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SalesCommissionTierColumns defines and stores column names for the table plugin_idc_core_sales_commission_tier.
type SalesCommissionTierColumns struct {
	Id              string //
	TenantId        string //
	TierNo          string //
	RuleNo          string //
	TierOrder       string //
	LowerBoundMinor string //
	UpperBoundMinor string //
	RateBasisPoints string //
	CreatedAt       string //
}

// salesCommissionTierColumns holds the columns for the table plugin_idc_core_sales_commission_tier.
var salesCommissionTierColumns = SalesCommissionTierColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	TierNo:          "tier_no",
	RuleNo:          "rule_no",
	TierOrder:       "tier_order",
	LowerBoundMinor: "lower_bound_minor",
	UpperBoundMinor: "upper_bound_minor",
	RateBasisPoints: "rate_basis_points",
	CreatedAt:       "created_at",
}

// NewSalesCommissionTierDao creates and returns a new DAO object for table data access.
func NewSalesCommissionTierDao(handlers ...gdb.ModelHandler) *SalesCommissionTierDao {
	return &SalesCommissionTierDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_tier",
		columns:  salesCommissionTierColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionTierDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionTierDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionTierDao) Columns() SalesCommissionTierColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionTierDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionTierDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionTierDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
