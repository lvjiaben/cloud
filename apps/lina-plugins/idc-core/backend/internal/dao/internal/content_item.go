// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ContentItemDao is the data access object for the table plugin_idc_core_content_item.
type ContentItemDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ContentItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ContentItemColumns defines and stores column names for the table plugin_idc_core_content_item.
type ContentItemColumns struct {
	Id              string //
	TenantId        string //
	ContentNo       string //
	ContentType     string //
	CategoryNo      string //
	VisibilityCode  string //
	Title           string //
	Summary         string //
	Body            string //
	SeoTitle        string //
	SeoDescription  string //
	PublicPath      string //
	StatusCode      string //
	ValidityVersion string //
	CurrentFileNo   string //
	PublishedAt     string //
	OfflineAt       string //
	DeletedAt       string //
	Version         string //
	CreatedBy       string //
	UpdatedBy       string //
	CreatedAt       string //
	UpdatedAt       string //
}

// contentItemColumns holds the columns for the table plugin_idc_core_content_item.
var contentItemColumns = ContentItemColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	ContentNo:       "content_no",
	ContentType:     "content_type",
	CategoryNo:      "category_no",
	VisibilityCode:  "visibility_code",
	Title:           "title",
	Summary:         "summary",
	Body:            "body",
	SeoTitle:        "seo_title",
	SeoDescription:  "seo_description",
	PublicPath:      "public_path",
	StatusCode:      "status_code",
	ValidityVersion: "validity_version",
	CurrentFileNo:   "current_file_no",
	PublishedAt:     "published_at",
	OfflineAt:       "offline_at",
	DeletedAt:       "deleted_at",
	Version:         "version",
	CreatedBy:       "created_by",
	UpdatedBy:       "updated_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewContentItemDao creates and returns a new DAO object for table data access.
func NewContentItemDao(handlers ...gdb.ModelHandler) *ContentItemDao {
	return &ContentItemDao{
		group:    "default",
		table:    "plugin_idc_core_content_item",
		columns:  contentItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ContentItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ContentItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ContentItemDao) Columns() ContentItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ContentItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ContentItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ContentItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
