// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersionDao is the data access object for the table plugin_idc_core_catalog_product_version.
type CatalogProductVersionDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  CatalogProductVersionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// CatalogProductVersionColumns defines and stores column names for the table plugin_idc_core_catalog_product_version.
type CatalogProductVersionColumns struct {
	Id                    string //
	TenantId              string //
	VersionNo             string //
	ProductNo             string //
	VersionNumber         string //
	StatusCode            string //
	SourceCode            string //
	SourceVersion         string //
	SourceFingerprint     string //
	ChangeSummary         string //
	RequiresContract      string //
	PublishedAt           string //
	RetiredAt             string //
	CreatedBy             string //
	CreatedAt             string //
	ContractPolicyCode    string // Product choice: inherit enterprise, explicit none, or required
	ContractTemplateNo    string //
	ContractPolicyVersion string //
}

// catalogProductVersionColumns holds the columns for the table plugin_idc_core_catalog_product_version.
var catalogProductVersionColumns = CatalogProductVersionColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	VersionNo:             "version_no",
	ProductNo:             "product_no",
	VersionNumber:         "version_number",
	StatusCode:            "status_code",
	SourceCode:            "source_code",
	SourceVersion:         "source_version",
	SourceFingerprint:     "source_fingerprint",
	ChangeSummary:         "change_summary",
	RequiresContract:      "requires_contract",
	PublishedAt:           "published_at",
	RetiredAt:             "retired_at",
	CreatedBy:             "created_by",
	CreatedAt:             "created_at",
	ContractPolicyCode:    "contract_policy_code",
	ContractTemplateNo:    "contract_template_no",
	ContractPolicyVersion: "contract_policy_version",
}

// NewCatalogProductVersionDao creates and returns a new DAO object for table data access.
func NewCatalogProductVersionDao(handlers ...gdb.ModelHandler) *CatalogProductVersionDao {
	return &CatalogProductVersionDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_product_version",
		columns:  catalogProductVersionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogProductVersionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogProductVersionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogProductVersionDao) Columns() CatalogProductVersionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogProductVersionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogProductVersionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogProductVersionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
