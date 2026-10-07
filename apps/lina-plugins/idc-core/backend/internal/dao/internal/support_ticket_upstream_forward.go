// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketUpstreamForwardDao is the data access object for the table plugin_idc_core_support_ticket_upstream_forward.
type SupportTicketUpstreamForwardDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  SupportTicketUpstreamForwardColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// SupportTicketUpstreamForwardColumns defines and stores column names for the table plugin_idc_core_support_ticket_upstream_forward.
type SupportTicketUpstreamForwardColumns struct {
	Id                      string //
	TenantId                string //
	ForwardNo               string //
	TicketNo                string //
	ProviderId              string //
	ProviderTicketReference string //
	SelectionDigest         string //
	PreviewDigest           string //
	StatusCode              string //
	ResultClassification    string //
	RetryEligible           string //
	ConfirmedBy             string //
	ConfirmedAt             string //
	Version                 string //
	CreatedAt               string //
	UpdatedAt               string //
	OperationNo             string //
	RequestDigest           string //
	UpstreamDepartmentId    string //
	EntitlementUpstreamRef  string //
	AttemptCount            string //
	LastAttemptAt           string //
	NextQueryAt             string //
	ConfirmedReason         string //
	LastEvidenceCode        string //
}

// supportTicketUpstreamForwardColumns holds the columns for the table plugin_idc_core_support_ticket_upstream_forward.
var supportTicketUpstreamForwardColumns = SupportTicketUpstreamForwardColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	ForwardNo:               "forward_no",
	TicketNo:                "ticket_no",
	ProviderId:              "provider_id",
	ProviderTicketReference: "provider_ticket_reference",
	SelectionDigest:         "selection_digest",
	PreviewDigest:           "preview_digest",
	StatusCode:              "status_code",
	ResultClassification:    "result_classification",
	RetryEligible:           "retry_eligible",
	ConfirmedBy:             "confirmed_by",
	ConfirmedAt:             "confirmed_at",
	Version:                 "version",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
	OperationNo:             "operation_no",
	RequestDigest:           "request_digest",
	UpstreamDepartmentId:    "upstream_department_id",
	EntitlementUpstreamRef:  "entitlement_upstream_ref",
	AttemptCount:            "attempt_count",
	LastAttemptAt:           "last_attempt_at",
	NextQueryAt:             "next_query_at",
	ConfirmedReason:         "confirmed_reason",
	LastEvidenceCode:        "last_evidence_code",
}

// NewSupportTicketUpstreamForwardDao creates and returns a new DAO object for table data access.
func NewSupportTicketUpstreamForwardDao(handlers ...gdb.ModelHandler) *SupportTicketUpstreamForwardDao {
	return &SupportTicketUpstreamForwardDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_upstream_forward",
		columns:  supportTicketUpstreamForwardColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketUpstreamForwardDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketUpstreamForwardDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketUpstreamForwardDao) Columns() SupportTicketUpstreamForwardColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketUpstreamForwardDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketUpstreamForwardDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketUpstreamForwardDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
