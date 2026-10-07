// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CreditApplicationDao is the data access object for the table plugin_idc_core_credit_application.
type CreditApplicationDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  CreditApplicationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// CreditApplicationColumns defines and stores column names for the table plugin_idc_core_credit_application.
type CreditApplicationColumns struct {
	Id                    string //
	TenantId              string //
	ApplicationNo         string //
	CustomerNo            string //
	CurrencyCode          string //
	PolicyNo              string //
	PolicyVersion         string //
	RequestedLimitMinor   string //
	RequestedValidFrom    string //
	RequestedValidUntil   string //
	RequiredApprovalCount string //
	RiskEvidenceReference string //
	RiskEvidenceDigest    string //
	RequestDigest         string //
	StatusCode            string //
	RequestActorTypeCode  string //
	RequestedBy           string //
	DecidedAt             string //
	CancelledAt           string //
	Version               string //
	CreatedAt             string //
	UpdatedAt             string //
}

// creditApplicationColumns holds the columns for the table plugin_idc_core_credit_application.
var creditApplicationColumns = CreditApplicationColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	ApplicationNo:         "application_no",
	CustomerNo:            "customer_no",
	CurrencyCode:          "currency_code",
	PolicyNo:              "policy_no",
	PolicyVersion:         "policy_version",
	RequestedLimitMinor:   "requested_limit_minor",
	RequestedValidFrom:    "requested_valid_from",
	RequestedValidUntil:   "requested_valid_until",
	RequiredApprovalCount: "required_approval_count",
	RiskEvidenceReference: "risk_evidence_reference",
	RiskEvidenceDigest:    "risk_evidence_digest",
	RequestDigest:         "request_digest",
	StatusCode:            "status_code",
	RequestActorTypeCode:  "request_actor_type_code",
	RequestedBy:           "requested_by",
	DecidedAt:             "decided_at",
	CancelledAt:           "cancelled_at",
	Version:               "version",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
}

// NewCreditApplicationDao creates and returns a new DAO object for table data access.
func NewCreditApplicationDao(handlers ...gdb.ModelHandler) *CreditApplicationDao {
	return &CreditApplicationDao{
		group:    "default",
		table:    "plugin_idc_core_credit_application",
		columns:  creditApplicationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CreditApplicationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CreditApplicationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CreditApplicationDao) Columns() CreditApplicationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CreditApplicationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CreditApplicationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CreditApplicationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
