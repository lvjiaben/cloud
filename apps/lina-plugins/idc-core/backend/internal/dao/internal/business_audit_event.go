// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BusinessAuditEventDao is the data access object for the table plugin_idc_core_business_audit_event.
type BusinessAuditEventDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  BusinessAuditEventColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// BusinessAuditEventColumns defines and stores column names for the table plugin_idc_core_business_audit_event.
type BusinessAuditEventColumns struct {
	Id                string //
	TenantId          string //
	AuditNo           string // Opaque audit event business number
	OccurredAt        string //
	ActorType         string // Authenticated user, system, or external actor type
	ActorUserId       string // Effective host identity responsible for the action
	ActingUserId      string // Actual host identity acting through assistance or delegation
	ActorAudienceCode string //
	ActorCustomerNo   string // Customer subject represented by the actor when the actor audience is customer
	CustomerNo        string // Validated customer subject affected by the audited operation
	OperationCode     string // Stable audited business operation code
	TargetType        string //
	TargetNo          string // Opaque target business number
	ResultCode        string //
	ReasonCode        string //
	RiskCode          string //
	BeforeSnapshot    string // Redacted state before the operation
	AfterSnapshot     string // Redacted state after the operation
	ContextSnapshot   string // Redacted permission, reason, and risk context
	RequestId         string //
	TraceId           string //
	CorrelationNo     string //
	IpDigest          string // Keyed digest of the source IP when policy permits
	UserAgentDigest   string // Digest of the user agent when policy permits
	CreatedAt         string //
}

// businessAuditEventColumns holds the columns for the table plugin_idc_core_business_audit_event.
var businessAuditEventColumns = BusinessAuditEventColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	AuditNo:           "audit_no",
	OccurredAt:        "occurred_at",
	ActorType:         "actor_type",
	ActorUserId:       "actor_user_id",
	ActingUserId:      "acting_user_id",
	ActorAudienceCode: "actor_audience_code",
	ActorCustomerNo:   "actor_customer_no",
	CustomerNo:        "customer_no",
	OperationCode:     "operation_code",
	TargetType:        "target_type",
	TargetNo:          "target_no",
	ResultCode:        "result_code",
	ReasonCode:        "reason_code",
	RiskCode:          "risk_code",
	BeforeSnapshot:    "before_snapshot",
	AfterSnapshot:     "after_snapshot",
	ContextSnapshot:   "context_snapshot",
	RequestId:         "request_id",
	TraceId:           "trace_id",
	CorrelationNo:     "correlation_no",
	IpDigest:          "ip_digest",
	UserAgentDigest:   "user_agent_digest",
	CreatedAt:         "created_at",
}

// NewBusinessAuditEventDao creates and returns a new DAO object for table data access.
func NewBusinessAuditEventDao(handlers ...gdb.ModelHandler) *BusinessAuditEventDao {
	return &BusinessAuditEventDao{
		group:    "default",
		table:    "plugin_idc_core_business_audit_event",
		columns:  businessAuditEventColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BusinessAuditEventDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BusinessAuditEventDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BusinessAuditEventDao) Columns() BusinessAuditEventColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BusinessAuditEventDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BusinessAuditEventDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BusinessAuditEventDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
