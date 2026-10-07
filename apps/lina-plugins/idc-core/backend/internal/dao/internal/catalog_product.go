// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductDao is the data access object for the table plugin_idc_core_catalog_product.
type CatalogProductDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  CatalogProductColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// CatalogProductColumns defines and stores column names for the table plugin_idc_core_catalog_product.
type CatalogProductColumns struct {
	Id               string //
	TenantId         string //
	ProductNo        string //
	CategoryNo       string //
	StatusCode       string //
	PublicSlug       string //
	NameZhCn         string //
	NameEnUs         string //
	SummaryZhCn      string //
	SummaryEnUs      string //
	CurrentVersionNo string //
	SortOrder        string //
	Version          string //
	CreatedBy        string //
	UpdatedBy        string //
	CreatedAt        string //
	UpdatedAt        string //
	DeletedAt        string //
}

// catalogProductColumns holds the columns for the table plugin_idc_core_catalog_product.
var catalogProductColumns = CatalogProductColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	ProductNo:        "product_no",
	CategoryNo:       "category_no",
	StatusCode:       "status_code",
	PublicSlug:       "public_slug",
	NameZhCn:         "name_zh_cn",
	NameEnUs:         "name_en_us",
	SummaryZhCn:      "summary_zh_cn",
	SummaryEnUs:      "summary_en_us",
	CurrentVersionNo: "current_version_no",
	SortOrder:        "sort_order",
	Version:          "version",
	CreatedBy:        "created_by",
	UpdatedBy:        "updated_by",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
	DeletedAt:        "deleted_at",
}

// NewCatalogProductDao creates and returns a new DAO object for table data access.
func NewCatalogProductDao(handlers ...gdb.ModelHandler) *CatalogProductDao {
	return &CatalogProductDao{
		group:    "default",
		table:    "plugin_idc_core_catalog_product",
		columns:  catalogProductColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CatalogProductDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CatalogProductDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CatalogProductDao) Columns() CatalogProductColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CatalogProductDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CatalogProductDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CatalogProductDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
