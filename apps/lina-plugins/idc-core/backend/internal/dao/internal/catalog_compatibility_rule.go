// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogCompatibilityRuleDao is the data access object for the table plugin_idc_core_catalog_compatibility_rule.
type CatalogCompatibilityRuleDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  CatalogCompatibilityRuleColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// CatalogCompatibilityRuleColumns defines and stores column names for the table plugin_idc_core_catalog_compatibility_rule.
type CatalogCompatibilityRuleColumns struct {
	Id                    string //
	TenantId              string //
	RuleNo                string //
	VersionNo             string //
	RuleTypeCode          string //
	SourceOptionNo        string //
	TargetOptionNo        string //
	MinimumTargetQuantity string //
	MaximumTargetQuantity string //
	StatusCode            string //
	CreatedAt             string //
}

// catalogCompatibilityRuleColumns holds the columns for the table plugin_idc_core_catalog_compatibility_rule.
var catalogCompatibilityRuleColumns = CatalogCompatibilityRuleColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	RuleNo:                "rule_no",
	VersionNo:             "version_no",
	RuleTypeCode:          "rule_type_code",
	SourceOptionNo:        "source_option_no",
	TargetOptionNo:        "target_option_no",
	MinimumTargetQuantity: "minimum_target_quantity",
	MaximumTargetQuantity: "maximum_target_quantity",
	StatusCode:            "status_code",
	CreatedAt:             "created_at",
}

// NewCatalogCompatibilityRuleDao creates and returns a new DAO object for table data access.
func NewCatalogCompatibilityRuleDao(handlers ...gdb.ModelHandler) *CatalogCompatibilityRuleDao {
	return &CatalogCompatibilityRuleDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_compatibility_rule",
		columns:  catalogCompatibilityRuleColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogCompatibilityRuleDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogCompatibilityRuleDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogCompatibilityRuleDao) Columns() CatalogCompatibilityRuleColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogCompatibilityRuleDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogCompatibilityRuleDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogCompatibilityRuleDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
