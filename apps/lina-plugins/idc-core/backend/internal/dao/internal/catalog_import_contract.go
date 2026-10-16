// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportContractDao is the data access object for the table plugin_idc_core_catalog_import_contract.
type CatalogImportContractDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  CatalogImportContractColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// CatalogImportContractColumns defines and stores column names for the table plugin_idc_core_catalog_import_contract.
type CatalogImportContractColumns struct {
	Id                  string //
	TenantId            string //
	VersionNo           string //
	InterfaceInstanceNo string //
	Snapshot            string //
	SnapshotDigest      string //
	ReadEvidence        string //
	CreatedAt           string //
}

// catalogImportContractColumns holds the columns for the table plugin_idc_core_catalog_import_contract.
var catalogImportContractColumns = CatalogImportContractColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	VersionNo:           "version_no",
	InterfaceInstanceNo: "interface_instance_no",
	Snapshot:            "snapshot",
	SnapshotDigest:      "snapshot_digest",
	ReadEvidence:        "read_evidence",
	CreatedAt:           "created_at",
}

// NewCatalogImportContractDao creates and returns a new DAO object for table data access.
func NewCatalogImportContractDao(handlers ...gdb.ModelHandler) *CatalogImportContractDao {
	return &CatalogImportContractDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_import_contract",
		columns:  catalogImportContractColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogImportContractDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogImportContractDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogImportContractDao) Columns() CatalogImportContractColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogImportContractDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogImportContractDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogImportContractDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
