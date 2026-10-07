// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// WecomEmployeeBindingDao is the data access object for the table plugin_idc_notify_wecom_employee_binding.
type WecomEmployeeBindingDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  WecomEmployeeBindingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// WecomEmployeeBindingColumns defines and stores column names for the table plugin_idc_notify_wecom_employee_binding.
type WecomEmployeeBindingColumns struct {
	TenantId                string //
	BindingNo               string //
	HostUserId              string //
	StaffVersion            string //
	CorpId                  string //
	ChannelGeneration       string //
	IdentityDigest          string //
	IdentitySecretRef       string //
	IdentitySecretUpdatedAt string //
	SecretOperationKey      string //
	StatusCode              string //
	TerminalStatusCode      string //
	RequestNo               string //
	AvailableAt             string //
	LeaseToken              string //
	LeaseExpiresAt          string //
	AttemptCount            string //
	LastErrorCode           string //
	Version                 string //
	BoundAt                 string //
	TerminalAt              string //
	CreatedAt               string //
	UpdatedAt               string //
}

// wecomEmployeeBindingColumns holds the columns for the table plugin_idc_notify_wecom_employee_binding.
var wecomEmployeeBindingColumns = WecomEmployeeBindingColumns{
	TenantId:                "tenant_id",
	BindingNo:               "binding_no",
	HostUserId:              "host_user_id",
	StaffVersion:            "staff_version",
	CorpId:                  "corp_id",
	ChannelGeneration:       "channel_generation",
	IdentityDigest:          "identity_digest",
	IdentitySecretRef:       "identity_secret_ref",
	IdentitySecretUpdatedAt: "identity_secret_updated_at",
	SecretOperationKey:      "secret_operation_key",
	StatusCode:              "status_code",
	TerminalStatusCode:      "terminal_status_code",
	RequestNo:               "request_no",
	AvailableAt:             "available_at",
	LeaseToken:              "lease_token",
	LeaseExpiresAt:          "lease_expires_at",
	AttemptCount:            "attempt_count",
	LastErrorCode:           "last_error_code",
	Version:                 "version",
	BoundAt:                 "bound_at",
	TerminalAt:              "terminal_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
}

// NewWecomEmployeeBindingDao creates and returns a new DAO object for table data access.
func NewWecomEmployeeBindingDao(handlers ...gdb.ModelHandler) *WecomEmployeeBindingDao {
	return &WecomEmployeeBindingDao{
		group:    "default",
		table:    "plugin_idc_notify_wecom_employee_binding",
		columns:  wecomEmployeeBindingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *WecomEmployeeBindingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *WecomEmployeeBindingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *WecomEmployeeBindingDao) Columns() WecomEmployeeBindingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *WecomEmployeeBindingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *WecomEmployeeBindingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *WecomEmployeeBindingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
