// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// InboxMessageDao is the data access object for the table plugin_shared_services_inbox_message.
type InboxMessageDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  InboxMessageColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// InboxMessageColumns defines and stores column names for the table plugin_shared_services_inbox_message.
type InboxMessageColumns struct {
	Id              string //
	TenantId        string //
	MessageNo       string //
	RecipientUserId string //
	CallerPluginId  string //
	SourceType      string //
	SourceNo        string //
	IdempotencyKey  string //
	ContentDigest   string //
	Title           string //
	Body            string //
	Category        string //
	ActionUrl       string //
	Status          string //
	ReadAt          string //
	CreatedAt       string //
	UpdatedAt       string //
}

// inboxMessageColumns holds the columns for the table plugin_shared_services_inbox_message.
var inboxMessageColumns = InboxMessageColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	MessageNo:       "message_no",
	RecipientUserId: "recipient_user_id",
	CallerPluginId:  "caller_plugin_id",
	SourceType:      "source_type",
	SourceNo:        "source_no",
	IdempotencyKey:  "idempotency_key",
	ContentDigest:   "content_digest",
	Title:           "title",
	Body:            "body",
	Category:        "category",
	ActionUrl:       "action_url",
	Status:          "status",
	ReadAt:          "read_at",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewInboxMessageDao creates and returns a new DAO object for table data access.
func NewInboxMessageDao(handlers ...gdb.ModelHandler) *InboxMessageDao {
	return &InboxMessageDao{
		group:    "default",
		table:    "plugin_shared_services_inbox_message",
		columns:  inboxMessageColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *InboxMessageDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *InboxMessageDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *InboxMessageDao) Columns() InboxMessageColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *InboxMessageDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *InboxMessageDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *InboxMessageDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
