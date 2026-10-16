// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportTermDao is the data access object for the table plugin_idc_core_catalog_import_term.
type CatalogImportTermDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  CatalogImportTermColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// CatalogImportTermColumns defines and stores column names for the table plugin_idc_core_catalog_import_term.
type CatalogImportTermColumns struct {
	Id                    string //
	TenantId              string //
	VersionNo             string //
	BillingRuleNo         string //
	CurrencyCode          string //
	UpstreamTermReference string //
	WholeItemRule         string //
	MarkupPercent         string //
	CreatedAt             string //
}

// catalogImportTermColumns holds the columns for the table plugin_idc_core_catalog_import_term.
var catalogImportTermColumns = CatalogImportTermColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	VersionNo:             "version_no",
	BillingRuleNo:         "billing_rule_no",
	CurrencyCode:          "currency_code",
	UpstreamTermReference: "upstream_term_reference",
	WholeItemRule:         "whole_item_rule",
	MarkupPercent:         "markup_percent",
	CreatedAt:             "created_at",
}

// NewCatalogImportTermDao creates and returns a new DAO object for table data access.
func NewCatalogImportTermDao(handlers ...gdb.ModelHandler) *CatalogImportTermDao {
	return &CatalogImportTermDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_import_term",
		columns:  catalogImportTermColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogImportTermDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogImportTermDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogImportTermDao) Columns() CatalogImportTermColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogImportTermDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogImportTermDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogImportTermDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
