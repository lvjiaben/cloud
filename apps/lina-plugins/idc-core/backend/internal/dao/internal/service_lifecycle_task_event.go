// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecycleTaskEventDao is the data access object for the table plugin_idc_core_service_lifecycle_task_event.
type ServiceLifecycleTaskEventDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  ServiceLifecycleTaskEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// ServiceLifecycleTaskEventColumns defines and stores column names for the table plugin_idc_core_service_lifecycle_task_event.
type ServiceLifecycleTaskEventColumns struct {
	Id             string //
	TenantId       string //
	EventNo        string //
	TaskNo         string //
	ServiceNo      string //
	EventSequence  string //
	FromStatusCode string //
	ToStatusCode   string //
	ReasonCode     string //
	ActorTypeCode  string //
	ActorReference string //
	EvidenceDigest string //
	ContentDigest  string //
	CreatedAt      string //
}

// serviceLifecycleTaskEventColumns holds the columns for the table plugin_idc_core_service_lifecycle_task_event.
var serviceLifecycleTaskEventColumns = ServiceLifecycleTaskEventColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	EventNo:        "event_no",
	TaskNo:         "task_no",
	ServiceNo:      "service_no",
	EventSequence:  "event_sequence",
	FromStatusCode: "from_status_code",
	ToStatusCode:   "to_status_code",
	ReasonCode:     "reason_code",
	ActorTypeCode:  "actor_type_code",
	ActorReference: "actor_reference",
	EvidenceDigest: "evidence_digest",
	ContentDigest:  "content_digest",
	CreatedAt:      "created_at",
}

// NewServiceLifecycleTaskEventDao creates and returns a new DAO object for table data access.
func NewServiceLifecycleTaskEventDao(handlers ...gdb.ModelHandler) *ServiceLifecycleTaskEventDao {
	return &ServiceLifecycleTaskEventDao{
		group:    "default",
		table:    "plugin_idc_core_service_lifecycle_task_event",
		columns:  serviceLifecycleTaskEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceLifecycleTaskEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceLifecycleTaskEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceLifecycleTaskEventDao) Columns() ServiceLifecycleTaskEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceLifecycleTaskEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceLifecycleTaskEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceLifecycleTaskEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
