// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelIntentDao is the data access object for the table plugin_idc_notify_channel_intent.
type ChannelIntentDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  ChannelIntentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// ChannelIntentColumns defines and stores column names for the table plugin_idc_notify_channel_intent.
type ChannelIntentColumns struct {
	TenantId                      string //
	OperationNo                   string //
	ChannelCode                   string //
	OperationCode                 string //
	StatusCode                    string //
	ActorUserId                   string //
	BaseStatusCode                string //
	BaseVersion                   string //
	RequestShapeDigest            string //
	DesiredProviderCode           string //
	DesiredNonSecretConfiguration string //
	SecretOperationKey            string //
	CredentialSecretRef           string //
	SecretExpectedUpdatedAt       string //
	SecretResultUpdatedAt         string //
	SecretResultMaskedTail        string //
	AvailableAt                   string //
	LeaseToken                    string //
	LeaseExpiresAt                string //
	AttemptCount                  string //
	LastErrorCode                 string //
	CompletedAt                   string //
	CreatedAt                     string //
	UpdatedAt                     string //
}

// channelIntentColumns holds the columns for the table plugin_idc_notify_channel_intent.
var channelIntentColumns = ChannelIntentColumns{
	TenantId:                      "tenant_id",
	OperationNo:                   "operation_no",
	ChannelCode:                   "channel_code",
	OperationCode:                 "operation_code",
	StatusCode:                    "status_code",
	ActorUserId:                   "actor_user_id",
	BaseStatusCode:                "base_status_code",
	BaseVersion:                   "base_version",
	RequestShapeDigest:            "request_shape_digest",
	DesiredProviderCode:           "desired_provider_code",
	DesiredNonSecretConfiguration: "desired_non_secret_configuration",
	SecretOperationKey:            "secret_operation_key",
	CredentialSecretRef:           "credential_secret_ref",
	SecretExpectedUpdatedAt:       "secret_expected_updated_at",
	SecretResultUpdatedAt:         "secret_result_updated_at",
	SecretResultMaskedTail:        "secret_result_masked_tail",
	AvailableAt:                   "available_at",
	LeaseToken:                    "lease_token",
	LeaseExpiresAt:                "lease_expires_at",
	AttemptCount:                  "attempt_count",
	LastErrorCode:                 "last_error_code",
	CompletedAt:                   "completed_at",
	CreatedAt:                     "created_at",
	UpdatedAt:                     "updated_at",
}

// NewChannelIntentDao creates and returns a new DAO object for table data access.
func NewChannelIntentDao(handlers ...gdb.ModelHandler) *ChannelIntentDao {
	return &ChannelIntentDao{
		group:    "default",
		table:    "plugin_idc_notify_channel_intent",
		columns:  channelIntentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelIntentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelIntentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelIntentDao) Columns() ChannelIntentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelIntentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelIntentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelIntentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
