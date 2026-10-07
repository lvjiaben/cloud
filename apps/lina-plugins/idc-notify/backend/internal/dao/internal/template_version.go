// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TemplateVersionDao is the data access object for the table plugin_idc_notify_template_version.
type TemplateVersionDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  TemplateVersionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// TemplateVersionColumns defines and stores column names for the table plugin_idc_notify_template_version.
type TemplateVersionColumns struct {
	TenantId                  string //
	TemplateNo                string //
	Version                   string //
	EventCode                 string //
	ChannelCode               string //
	LocaleCode                string //
	SubjectTemplate           string //
	BodyTemplate              string //
	ProviderTemplateId        string //
	ProviderSmsVariableOrder  string //
	ProviderWechatDataMapping string //
	StatusCode                string //
	ContentDigest             string //
	CreatedBy                 string //
	CreatedAt                 string //
	ProviderCode              string //
	ProviderSmsDataMapping    string //
}

// templateVersionColumns holds the columns for the table plugin_idc_notify_template_version.
var templateVersionColumns = TemplateVersionColumns{
	TenantId:                  "tenant_id",
	TemplateNo:                "template_no",
	Version:                   "version",
	EventCode:                 "event_code",
	ChannelCode:               "channel_code",
	LocaleCode:                "locale_code",
	SubjectTemplate:           "subject_template",
	BodyTemplate:              "body_template",
	ProviderTemplateId:        "provider_template_id",
	ProviderSmsVariableOrder:  "provider_sms_variable_order",
	ProviderWechatDataMapping: "provider_wechat_data_mapping",
	StatusCode:                "status_code",
	ContentDigest:             "content_digest",
	CreatedBy:                 "created_by",
	CreatedAt:                 "created_at",
	ProviderCode:              "provider_code",
	ProviderSmsDataMapping:    "provider_sms_data_mapping",
}

// NewTemplateVersionDao creates and returns a new DAO object for table data access.
func NewTemplateVersionDao(handlers ...gdb.ModelHandler) *TemplateVersionDao {
	return &TemplateVersionDao{
		group:    "default",
		table:    "plugin_idc_notify_template_version",
		columns:  templateVersionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TemplateVersionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TemplateVersionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TemplateVersionDao) Columns() TemplateVersionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TemplateVersionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TemplateVersionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TemplateVersionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
