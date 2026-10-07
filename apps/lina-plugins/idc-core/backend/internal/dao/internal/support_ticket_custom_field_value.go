// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketCustomFieldValueDao is the data access object for the table plugin_idc_core_support_ticket_custom_field_value.
type SupportTicketCustomFieldValueDao struct {
	table    string                               // table is the underlying table name of the DAO.
	group    string                               // group is the database configuration group name of the current DAO.
	columns  SupportTicketCustomFieldValueColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                   // handlers for customized model modification.
}

// SupportTicketCustomFieldValueColumns defines and stores column names for the table plugin_idc_core_support_ticket_custom_field_value.
type SupportTicketCustomFieldValueColumns struct {
	Id                  string //
	TenantId            string //
	TicketNo            string //
	FieldNo             string //
	DefinitionVersion   string //
	FieldType           string //
	NameZhCn            string //
	NameEnUs            string //
	DescriptionZhCn     string //
	DescriptionEnUs     string //
	VisibilityCode      string //
	Required            string //
	SortOrder           string //
	NormalizedValue     string //
	SelectedOptionCount string //
	CreatedAt           string //
}

// supportTicketCustomFieldValueColumns holds the columns for the table plugin_idc_core_support_ticket_custom_field_value.
var supportTicketCustomFieldValueColumns = SupportTicketCustomFieldValueColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	TicketNo:            "ticket_no",
	FieldNo:             "field_no",
	DefinitionVersion:   "definition_version",
	FieldType:           "field_type",
	NameZhCn:            "name_zh_cn",
	NameEnUs:            "name_en_us",
	DescriptionZhCn:     "description_zh_cn",
	DescriptionEnUs:     "description_en_us",
	VisibilityCode:      "visibility_code",
	Required:            "required",
	SortOrder:           "sort_order",
	NormalizedValue:     "normalized_value",
	SelectedOptionCount: "selected_option_count",
	CreatedAt:           "created_at",
}

// NewSupportTicketCustomFieldValueDao creates and returns a new DAO object for table data access.
func NewSupportTicketCustomFieldValueDao(handlers ...gdb.ModelHandler) *SupportTicketCustomFieldValueDao {
	return &SupportTicketCustomFieldValueDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_custom_field_value",
		columns:  supportTicketCustomFieldValueColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketCustomFieldValueDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketCustomFieldValueDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketCustomFieldValueDao) Columns() SupportTicketCustomFieldValueColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketCustomFieldValueDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketCustomFieldValueDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketCustomFieldValueDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
