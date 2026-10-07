// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionRuleDao is the data access object for the table plugin_idc_core_sales_commission_rule.
type SalesCommissionRuleDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SalesCommissionRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SalesCommissionRuleColumns defines and stores column names for the table plugin_idc_core_sales_commission_rule.
type SalesCommissionRuleColumns struct {
	Id                  string //
	TenantId            string //
	RuleNo              string //
	BeneficiaryTypeCode string //
	OperationTypeCode   string //
	ProductNo           string //
	CurrencyCode        string //
	CalculationTypeCode string //
	FixedAmountMinor    string //
	RateBasisPoints     string //
	CapAmountMinor      string //
	MaturityDays        string //
	Priority            string //
	StatusCode          string //
	EffectiveAt         string //
	ExpiresAt           string //
	Version             string //
	RuleDigest          string //
	CreatedAt           string //
}

// salesCommissionRuleColumns holds the columns for the table plugin_idc_core_sales_commission_rule.
var salesCommissionRuleColumns = SalesCommissionRuleColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	RuleNo:              "rule_no",
	BeneficiaryTypeCode: "beneficiary_type_code",
	OperationTypeCode:   "operation_type_code",
	ProductNo:           "product_no",
	CurrencyCode:        "currency_code",
	CalculationTypeCode: "calculation_type_code",
	FixedAmountMinor:    "fixed_amount_minor",
	RateBasisPoints:     "rate_basis_points",
	CapAmountMinor:      "cap_amount_minor",
	MaturityDays:        "maturity_days",
	Priority:            "priority",
	StatusCode:          "status_code",
	EffectiveAt:         "effective_at",
	ExpiresAt:           "expires_at",
	Version:             "version",
	RuleDigest:          "rule_digest",
	CreatedAt:           "created_at",
}

// NewSalesCommissionRuleDao creates and returns a new DAO object for table data access.
func NewSalesCommissionRuleDao(handlers ...gdb.ModelHandler) *SalesCommissionRuleDao {
	return &SalesCommissionRuleDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_rule",
		columns:  salesCommissionRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionRuleDao) Columns() SalesCommissionRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
