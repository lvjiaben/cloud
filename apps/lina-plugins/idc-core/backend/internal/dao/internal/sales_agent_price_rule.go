// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentPriceRuleDao is the data access object for the table plugin_idc_core_sales_agent_price_rule.
type SalesAgentPriceRuleDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SalesAgentPriceRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SalesAgentPriceRuleColumns defines and stores column names for the table plugin_idc_core_sales_agent_price_rule.
type SalesAgentPriceRuleColumns struct {
	Id                  string //
	TenantId            string //
	PriceRuleNo         string //
	ProductRuleNo       string //
	CurrencyCode        string //
	AdjustmentTypeCode  string //
	FixedPriceMinor     string //
	DiscountBasisPoints string //
	MinimumMarginMinor  string //
	StatusCode          string //
	EffectiveAt         string //
	ExpiresAt           string //
	Version             string //
	RuleDigest          string //
	CreatedAt           string //
}

// salesAgentPriceRuleColumns holds the columns for the table plugin_idc_core_sales_agent_price_rule.
var salesAgentPriceRuleColumns = SalesAgentPriceRuleColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	PriceRuleNo:         "price_rule_no",
	ProductRuleNo:       "product_rule_no",
	CurrencyCode:        "currency_code",
	AdjustmentTypeCode:  "adjustment_type_code",
	FixedPriceMinor:     "fixed_price_minor",
	DiscountBasisPoints: "discount_basis_points",
	MinimumMarginMinor:  "minimum_margin_minor",
	StatusCode:          "status_code",
	EffectiveAt:         "effective_at",
	ExpiresAt:           "expires_at",
	Version:             "version",
	RuleDigest:          "rule_digest",
	CreatedAt:           "created_at",
}

// NewSalesAgentPriceRuleDao creates and returns a new DAO object for table data access.
func NewSalesAgentPriceRuleDao(handlers ...gdb.ModelHandler) *SalesAgentPriceRuleDao {
	return &SalesAgentPriceRuleDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_price_rule",
		columns:  salesAgentPriceRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentPriceRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentPriceRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentPriceRuleDao) Columns() SalesAgentPriceRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentPriceRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentPriceRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentPriceRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
