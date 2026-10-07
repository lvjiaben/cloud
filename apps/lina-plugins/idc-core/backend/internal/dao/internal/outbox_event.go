// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OutboxEventDao is the data access object for the table plugin_idc_core_outbox_event.
type OutboxEventDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  OutboxEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// OutboxEventColumns defines and stores column names for the table plugin_idc_core_outbox_event.
type OutboxEventColumns struct {
	Id                  string //
	TenantId            string //
	EventNo             string // Opaque outbox event business number
	EventType           string //
	EventVersion        string // Positive version of the published event contract
	Topic               string // Stable logical delivery topic
	AggregateType       string //
	AggregateNo         string //
	CorrelationNo       string // Opaque business correlation number
	DeduplicationDigest string // Digest that prevents duplicate events of the same type
	StatusCode          string //
	Payload             string // Versioned redacted event payload without secrets
	Metadata            string // Versioned delivery metadata without secrets
	OccurredAt          string //
	AvailableAt         string // Earliest time at which the event may be claimed
	LockedBy            string //
	LeaseExpiresAt      string // Claim lease expiry used for crash recovery
	AttemptCount        string //
	MaxAttempts         string //
	PublishedAt         string //
	LastErrorCode       string //
	LastErrorSummary    string //
	CreatedAt           string //
	UpdatedAt           string //
}

// outboxEventColumns holds the columns for the table plugin_idc_core_outbox_event.
var outboxEventColumns = OutboxEventColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	EventNo:             "event_no",
	EventType:           "event_type",
	EventVersion:        "event_version",
	Topic:               "topic",
	AggregateType:       "aggregate_type",
	AggregateNo:         "aggregate_no",
	CorrelationNo:       "correlation_no",
	DeduplicationDigest: "deduplication_digest",
	StatusCode:          "status_code",
	Payload:             "payload",
	Metadata:            "metadata",
	OccurredAt:          "occurred_at",
	AvailableAt:         "available_at",
	LockedBy:            "locked_by",
	LeaseExpiresAt:      "lease_expires_at",
	AttemptCount:        "attempt_count",
	MaxAttempts:         "max_attempts",
	PublishedAt:         "published_at",
	LastErrorCode:       "last_error_code",
	LastErrorSummary:    "last_error_summary",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewOutboxEventDao creates and returns a new DAO object for table data access.
func NewOutboxEventDao(handlers ...gdb.ModelHandler) *OutboxEventDao {
	return &OutboxEventDao{
		group:    "default",
		table:    "plugin_idc_core_outbox_event",
		columns:  outboxEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OutboxEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OutboxEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OutboxEventDao) Columns() OutboxEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OutboxEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OutboxEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OutboxEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
