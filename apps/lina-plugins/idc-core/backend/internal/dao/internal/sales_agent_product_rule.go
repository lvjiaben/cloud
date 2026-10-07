// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentProductRuleDao is the data access object for the table plugin_idc_core_sales_agent_product_rule.
type SalesAgentProductRuleDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SalesAgentProductRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SalesAgentProductRuleColumns defines and stores column names for the table plugin_idc_core_sales_agent_product_rule.
type SalesAgentProductRuleColumns struct {
	Id            string //
	TenantId      string //
	ProductRuleNo string //
	LevelNo       string //
	ProductNo     string //
	StatusCode    string //
	EffectiveAt   string //
	ExpiresAt     string //
	Version       string //
	RuleDigest    string //
	CreatedAt     string //
}

// salesAgentProductRuleColumns holds the columns for the table plugin_idc_core_sales_agent_product_rule.
var salesAgentProductRuleColumns = SalesAgentProductRuleColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	ProductRuleNo: "product_rule_no",
	LevelNo:       "level_no",
	ProductNo:     "product_no",
	StatusCode:    "status_code",
	EffectiveAt:   "effective_at",
	ExpiresAt:     "expires_at",
	Version:       "version",
	RuleDigest:    "rule_digest",
	CreatedAt:     "created_at",
}

// NewSalesAgentProductRuleDao creates and returns a new DAO object for table data access.
func NewSalesAgentProductRuleDao(handlers ...gdb.ModelHandler) *SalesAgentProductRuleDao {
	return &SalesAgentProductRuleDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_product_rule",
		columns:  salesAgentProductRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentProductRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentProductRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentProductRuleDao) Columns() SalesAgentProductRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentProductRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentProductRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentProductRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
