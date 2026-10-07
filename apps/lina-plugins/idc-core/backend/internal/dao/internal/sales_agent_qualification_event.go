// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentQualificationEventDao is the data access object for the table plugin_idc_core_sales_agent_qualification_event.
type SalesAgentQualificationEventDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  SalesAgentQualificationEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// SalesAgentQualificationEventColumns defines and stores column names for the table plugin_idc_core_sales_agent_qualification_event.
type SalesAgentQualificationEventColumns struct {
	Id              string //
	TenantId        string //
	EventNo         string //
	QualificationNo string //
	EventSequence   string //
	FromStatusCode  string //
	ToStatusCode    string //
	FromLevelNo     string //
	ToLevelNo       string //
	ReasonCode      string //
	ActorReference  string //
	EvidenceDigest  string //
	OccurredAt      string //
}

// salesAgentQualificationEventColumns holds the columns for the table plugin_idc_core_sales_agent_qualification_event.
var salesAgentQualificationEventColumns = SalesAgentQualificationEventColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	EventNo:         "event_no",
	QualificationNo: "qualification_no",
	EventSequence:   "event_sequence",
	FromStatusCode:  "from_status_code",
	ToStatusCode:    "to_status_code",
	FromLevelNo:     "from_level_no",
	ToLevelNo:       "to_level_no",
	ReasonCode:      "reason_code",
	ActorReference:  "actor_reference",
	EvidenceDigest:  "evidence_digest",
	OccurredAt:      "occurred_at",
}

// NewSalesAgentQualificationEventDao creates and returns a new DAO object for table data access.
func NewSalesAgentQualificationEventDao(handlers ...gdb.ModelHandler) *SalesAgentQualificationEventDao {
	return &SalesAgentQualificationEventDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_qualification_event",
		columns:  salesAgentQualificationEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentQualificationEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentQualificationEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentQualificationEventDao) Columns() SalesAgentQualificationEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentQualificationEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentQualificationEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentQualificationEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
