// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelTestOperationDao is the data access object for the table plugin_idc_notify_channel_test_operation.
type ChannelTestOperationDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  ChannelTestOperationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// ChannelTestOperationColumns defines and stores column names for the table plugin_idc_notify_channel_test_operation.
type ChannelTestOperationColumns struct {
	TenantId                string //
	OperationNo             string //
	ChannelCode             string //
	ChannelVersion          string //
	TemplateNo              string //
	TemplateVersion         string //
	TargetMask              string //
	RequestDigest           string //
	StatusCode              string //
	ResultClass             string //
	CrossedExternalBoundary string //
	ProviderReference       string //
	ActorUserId             string //
	CompletedAt             string //
	CreatedAt               string //
	UpdatedAt               string //
}

// channelTestOperationColumns holds the columns for the table plugin_idc_notify_channel_test_operation.
var channelTestOperationColumns = ChannelTestOperationColumns{
	TenantId:                "tenant_id",
	OperationNo:             "operation_no",
	ChannelCode:             "channel_code",
	ChannelVersion:          "channel_version",
	TemplateNo:              "template_no",
	TemplateVersion:         "template_version",
	TargetMask:              "target_mask",
	RequestDigest:           "request_digest",
	StatusCode:              "status_code",
	ResultClass:             "result_class",
	CrossedExternalBoundary: "crossed_external_boundary",
	ProviderReference:       "provider_reference",
	ActorUserId:             "actor_user_id",
	CompletedAt:             "completed_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewChannelTestOperationDao creates and returns a new DAO object for table data access.
func NewChannelTestOperationDao(handlers ...gdb.ModelHandler) *ChannelTestOperationDao {
	return &ChannelTestOperationDao{
		group:    "default",
		table:    "plugin_idc_notify_channel_test_operation",
		columns:  channelTestOperationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelTestOperationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelTestOperationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelTestOperationDao) Columns() ChannelTestOperationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelTestOperationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelTestOperationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelTestOperationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
