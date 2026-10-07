// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentCreditLimitDao is the data access object for the table plugin_idc_core_sales_agent_credit_limit.
type SalesAgentCreditLimitDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SalesAgentCreditLimitColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SalesAgentCreditLimitColumns defines and stores column names for the table plugin_idc_core_sales_agent_credit_limit.
type SalesAgentCreditLimitColumns struct {
	Id               string //
	TenantId         string //
	LimitNo          string //
	QualificationNo  string //
	CurrencyCode     string //
	LimitAmountMinor string //
	StatusCode       string //
	EffectiveAt      string //
	ExpiresAt        string //
	CreatedAt        string //
}

// salesAgentCreditLimitColumns holds the columns for the table plugin_idc_core_sales_agent_credit_limit.
var salesAgentCreditLimitColumns = SalesAgentCreditLimitColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	LimitNo:          "limit_no",
	QualificationNo:  "qualification_no",
	CurrencyCode:     "currency_code",
	LimitAmountMinor: "limit_amount_minor",
	StatusCode:       "status_code",
	EffectiveAt:      "effective_at",
	ExpiresAt:        "expires_at",
	CreatedAt:        "created_at",
}

// NewSalesAgentCreditLimitDao creates and returns a new DAO object for table data access.
func NewSalesAgentCreditLimitDao(handlers ...gdb.ModelHandler) *SalesAgentCreditLimitDao {
	return &SalesAgentCreditLimitDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_credit_limit",
		columns:  salesAgentCreditLimitColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentCreditLimitDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentCreditLimitDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentCreditLimitDao) Columns() SalesAgentCreditLimitColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentCreditLimitDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentCreditLimitDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentCreditLimitDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
