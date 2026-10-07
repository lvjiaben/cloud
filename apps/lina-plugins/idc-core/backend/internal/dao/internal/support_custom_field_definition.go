// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportCustomFieldDefinitionDao is the data access object for the table plugin_idc_core_support_custom_field_definition.
type SupportCustomFieldDefinitionDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  SupportCustomFieldDefinitionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// SupportCustomFieldDefinitionColumns defines and stores column names for the table plugin_idc_core_support_custom_field_definition.
type SupportCustomFieldDefinitionColumns struct {
	Id              string //
	TenantId        string //
	FieldNo         string //
	FieldType       string //
	NameZhCn        string //
	NameEnUs        string //
	DescriptionZhCn string //
	DescriptionEnUs string //
	AppliesPresales string //
	AppliesCustomer string //
	AppliesInternal string //
	VisibilityCode  string //
	Required        string //
	SortOrder       string //
	StatusCode      string //
	Version         string //
	CreatedBy       string //
	UpdatedBy       string //
	CreatedAt       string //
	UpdatedAt       string //
}

// supportCustomFieldDefinitionColumns holds the columns for the table plugin_idc_core_support_custom_field_definition.
var supportCustomFieldDefinitionColumns = SupportCustomFieldDefinitionColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	FieldNo:         "field_no",
	FieldType:       "field_type",
	NameZhCn:        "name_zh_cn",
	NameEnUs:        "name_en_us",
	DescriptionZhCn: "description_zh_cn",
	DescriptionEnUs: "description_en_us",
	AppliesPresales: "applies_presales",
	AppliesCustomer: "applies_customer",
	AppliesInternal: "applies_internal",
	VisibilityCode:  "visibility_code",
	Required:        "required",
	SortOrder:       "sort_order",
	StatusCode:      "status_code",
	Version:         "version",
	CreatedBy:       "created_by",
	UpdatedBy:       "updated_by",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewSupportCustomFieldDefinitionDao creates and returns a new DAO object for table data access.
func NewSupportCustomFieldDefinitionDao(handlers ...gdb.ModelHandler) *SupportCustomFieldDefinitionDao {
	return &SupportCustomFieldDefinitionDao{
		group:    "default",
		table:    "plugin_idc_core_support_custom_field_definition",
		columns:  supportCustomFieldDefinitionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportCustomFieldDefinitionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportCustomFieldDefinitionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportCustomFieldDefinitionDao) Columns() SupportCustomFieldDefinitionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportCustomFieldDefinitionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportCustomFieldDefinitionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportCustomFieldDefinitionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
