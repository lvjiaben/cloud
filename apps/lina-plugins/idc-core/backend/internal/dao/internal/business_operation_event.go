// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BusinessOperationEventDao is the data access object for the table plugin_idc_core_business_operation_event.
type BusinessOperationEventDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  BusinessOperationEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// BusinessOperationEventColumns defines and stores column names for the table plugin_idc_core_business_operation_event.
type BusinessOperationEventColumns struct {
	Id              string //
	TenantId        string //
	EventNo         string //
	OperationNo     string //
	ServiceNo       string //
	EventSequence   string //
	FromStatusCode  string //
	ToStatusCode    string //
	ReasonCode      string //
	ActorTypeCode   string //
	ActorReference  string //
	EvidenceDigest  string //
	ContentDigest   string //
	CreatedAt       string //
	SubjectTypeCode string //
	SubjectNo       string //
}

// businessOperationEventColumns holds the columns for the table plugin_idc_core_business_operation_event.
var businessOperationEventColumns = BusinessOperationEventColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	EventNo:         "event_no",
	OperationNo:     "operation_no",
	ServiceNo:       "service_no",
	EventSequence:   "event_sequence",
	FromStatusCode:  "from_status_code",
	ToStatusCode:    "to_status_code",
	ReasonCode:      "reason_code",
	ActorTypeCode:   "actor_type_code",
	ActorReference:  "actor_reference",
	EvidenceDigest:  "evidence_digest",
	ContentDigest:   "content_digest",
	CreatedAt:       "created_at",
	SubjectTypeCode: "subject_type_code",
	SubjectNo:       "subject_no",
}

// NewBusinessOperationEventDao creates and returns a new DAO object for table data access.
func NewBusinessOperationEventDao(handlers ...gdb.ModelHandler) *BusinessOperationEventDao {
	return &BusinessOperationEventDao{
		group:    "default",
		table:    "plugin_idc_core_business_operation_event",
		columns:  businessOperationEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BusinessOperationEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BusinessOperationEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BusinessOperationEventDao) Columns() BusinessOperationEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BusinessOperationEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BusinessOperationEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BusinessOperationEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
