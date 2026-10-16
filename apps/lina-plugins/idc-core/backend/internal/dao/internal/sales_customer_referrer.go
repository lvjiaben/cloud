// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCustomerReferrerDao is the data access object for the table plugin_idc_core_sales_customer_referrer.
type SalesCustomerReferrerDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SalesCustomerReferrerColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SalesCustomerReferrerColumns defines and stores column names for the table plugin_idc_core_sales_customer_referrer.
type SalesCustomerReferrerColumns struct {
	Id                 string //
	TenantId           string //
	CustomerNo         string //
	ReferrerCustomerNo string //
	Version            string //
	EffectiveAt        string //
	CreatedBy          string //
}

// salesCustomerReferrerColumns holds the columns for the table plugin_idc_core_sales_customer_referrer.
var salesCustomerReferrerColumns = SalesCustomerReferrerColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	CustomerNo:         "customer_no",
	ReferrerCustomerNo: "referrer_customer_no",
	Version:            "version",
	EffectiveAt:        "effective_at",
	CreatedBy:          "created_by",
}

// NewSalesCustomerReferrerDao creates and returns a new DAO object for table data access.
func NewSalesCustomerReferrerDao(handlers ...gdb.ModelHandler) *SalesCustomerReferrerDao {
	return &SalesCustomerReferrerDao{
		group:    "default",
		table:    "plugin_idc_core_sales_customer_referrer",
		columns:  salesCustomerReferrerColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCustomerReferrerDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCustomerReferrerDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCustomerReferrerDao) Columns() SalesCustomerReferrerColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCustomerReferrerDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCustomerReferrerDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCustomerReferrerDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
