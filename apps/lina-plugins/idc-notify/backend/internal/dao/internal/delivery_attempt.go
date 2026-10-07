// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryAttemptDao is the data access object for the table plugin_idc_notify_delivery_attempt.
type DeliveryAttemptDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  DeliveryAttemptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// DeliveryAttemptColumns defines and stores column names for the table plugin_idc_notify_delivery_attempt.
type DeliveryAttemptColumns struct {
	TenantId                string //
	AttemptNo               string //
	TaskNo                  string //
	RetrySequence           string //
	AttemptFence            string //
	CrossedExternalBoundary string //
	ResultClass             string //
	EvidenceDigest          string //
	ProviderReference       string //
	StartedAt               string //
	FinishedAt              string //
}

// deliveryAttemptColumns holds the columns for the table plugin_idc_notify_delivery_attempt.
var deliveryAttemptColumns = DeliveryAttemptColumns{
	TenantId:                "tenant_id",
	AttemptNo:               "attempt_no",
	TaskNo:                  "task_no",
	RetrySequence:           "retry_sequence",
	AttemptFence:            "attempt_fence",
	CrossedExternalBoundary: "crossed_external_boundary",
	ResultClass:             "result_class",
	EvidenceDigest:          "evidence_digest",
	ProviderReference:       "provider_reference",
	StartedAt:               "started_at",
	FinishedAt:              "finished_at",
}

// NewDeliveryAttemptDao creates and returns a new DAO object for table data access.
func NewDeliveryAttemptDao(handlers ...gdb.ModelHandler) *DeliveryAttemptDao {
	return &DeliveryAttemptDao{
		group:    "default",
		table:    "plugin_idc_notify_delivery_attempt",
		columns:  deliveryAttemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *DeliveryAttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *DeliveryAttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *DeliveryAttemptDao) Columns() DeliveryAttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *DeliveryAttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *DeliveryAttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *DeliveryAttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
