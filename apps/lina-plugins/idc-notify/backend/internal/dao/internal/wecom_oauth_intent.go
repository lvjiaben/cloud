// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// WecomOauthIntentDao is the data access object for the table plugin_idc_notify_wecom_oauth_intent.
type WecomOauthIntentDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  WecomOauthIntentColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// WecomOauthIntentColumns defines and stores column names for the table plugin_idc_notify_wecom_oauth_intent.
type WecomOauthIntentColumns struct {
	TenantId          string //
	IntentNo          string //
	StateDigest       string //
	HostUserId        string //
	StaffVersion      string //
	SessionDigest     string //
	ChannelGeneration string //
	ChannelVersion    string //
	CorpId            string //
	AgentId           string //
	StatusCode        string //
	FailureCode       string //
	LeaseToken        string //
	LeaseExpiresAt    string //
	ExpiresAt         string //
	CompletedAt       string //
	Version           string //
	CreatedAt         string //
	UpdatedAt         string //
}

// wecomOauthIntentColumns holds the columns for the table plugin_idc_notify_wecom_oauth_intent.
var wecomOauthIntentColumns = WecomOauthIntentColumns{
	TenantId:          "tenant_id",
	IntentNo:          "intent_no",
	StateDigest:       "state_digest",
	HostUserId:        "host_user_id",
	StaffVersion:      "staff_version",
	SessionDigest:     "session_digest",
	ChannelGeneration: "channel_generation",
	ChannelVersion:    "channel_version",
	CorpId:            "corp_id",
	AgentId:           "agent_id",
	StatusCode:        "status_code",
	FailureCode:       "failure_code",
	LeaseToken:        "lease_token",
	LeaseExpiresAt:    "lease_expires_at",
	ExpiresAt:         "expires_at",
	CompletedAt:       "completed_at",
	Version:           "version",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewWecomOauthIntentDao creates and returns a new DAO object for table data access.
func NewWecomOauthIntentDao(handlers ...gdb.ModelHandler) *WecomOauthIntentDao {
	return &WecomOauthIntentDao{
		group:    "default",
		table:    "plugin_idc_notify_wecom_oauth_intent",
		columns:  wecomOauthIntentColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *WecomOauthIntentDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *WecomOauthIntentDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *WecomOauthIntentDao) Columns() WecomOauthIntentColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *WecomOauthIntentDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *WecomOauthIntentDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *WecomOauthIntentDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
