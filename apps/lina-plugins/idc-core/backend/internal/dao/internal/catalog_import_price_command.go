// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportPriceCommandDao is the data access object for the table plugin_idc_core_catalog_import_price_command.
type CatalogImportPriceCommandDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  CatalogImportPriceCommandColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// CatalogImportPriceCommandColumns defines and stores column names for the table plugin_idc_core_catalog_import_price_command.
type CatalogImportPriceCommandColumns struct {
	Id            string //
	TenantId      string //
	ActorUserId   string //
	RequestKey    string //
	RequestDigest string //
	VersionNo     string //
	CreatedAt     string //
}

// catalogImportPriceCommandColumns holds the columns for the table plugin_idc_core_catalog_import_price_command.
var catalogImportPriceCommandColumns = CatalogImportPriceCommandColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	ActorUserId:   "actor_user_id",
	RequestKey:    "request_key",
	RequestDigest: "request_digest",
	VersionNo:     "version_no",
	CreatedAt:     "created_at",
}

// NewCatalogImportPriceCommandDao creates and returns a new DAO object for table data access.
func NewCatalogImportPriceCommandDao(handlers ...gdb.ModelHandler) *CatalogImportPriceCommandDao {
	return &CatalogImportPriceCommandDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_import_price_command",
		columns:  catalogImportPriceCommandColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogImportPriceCommandDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogImportPriceCommandDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogImportPriceCommandDao) Columns() CatalogImportPriceCommandColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogImportPriceCommandDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogImportPriceCommandDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogImportPriceCommandDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
