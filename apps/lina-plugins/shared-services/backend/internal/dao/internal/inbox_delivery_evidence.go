// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// InboxDeliveryEvidenceDao is the data access object for the table plugin_shared_services_inbox_delivery_evidence.
type InboxDeliveryEvidenceDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  InboxDeliveryEvidenceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// InboxDeliveryEvidenceColumns defines and stores column names for the table plugin_shared_services_inbox_delivery_evidence.
type InboxDeliveryEvidenceColumns struct {
	Id             string //
	TenantId       string //
	EvidenceNo     string //
	MessageNo      string //
	CallerPluginId string //
	EvidenceType   string //
	ContentDigest  string //
	CreatedAt      string //
}

// inboxDeliveryEvidenceColumns holds the columns for the table plugin_shared_services_inbox_delivery_evidence.
var inboxDeliveryEvidenceColumns = InboxDeliveryEvidenceColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	EvidenceNo:     "evidence_no",
	MessageNo:      "message_no",
	CallerPluginId: "caller_plugin_id",
	EvidenceType:   "evidence_type",
	ContentDigest:  "content_digest",
	CreatedAt:      "created_at",
}

// NewInboxDeliveryEvidenceDao creates and returns a new DAO object for table data access.
func NewInboxDeliveryEvidenceDao(handlers ...gdb.ModelHandler) *InboxDeliveryEvidenceDao {
	return &InboxDeliveryEvidenceDao{
		group:    "default",
		table:    "plugin_shared_services_inbox_delivery_evidence",
		columns:  inboxDeliveryEvidenceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *InboxDeliveryEvidenceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *InboxDeliveryEvidenceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *InboxDeliveryEvidenceDao) Columns() InboxDeliveryEvidenceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *InboxDeliveryEvidenceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *InboxDeliveryEvidenceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *InboxDeliveryEvidenceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
