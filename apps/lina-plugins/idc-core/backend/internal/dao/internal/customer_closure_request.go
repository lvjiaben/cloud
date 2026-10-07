// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerClosureRequestDao is the data access object for the table plugin_idc_core_customer_closure_request.
type CustomerClosureRequestDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  CustomerClosureRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// CustomerClosureRequestColumns defines and stores column names for the table plugin_idc_core_customer_closure_request.
type CustomerClosureRequestColumns struct {
	Id                       string //
	TenantId                 string //
	RequestNo                string //
	CustomerNo               string //
	ConsentNo                string //
	StatusCode               string //
	SubjectVersion           string //
	UnsettledFundsCount      string //
	ActiveServiceCount       string //
	UnsettledInvoiceCount    string //
	ActiveContractCount      string //
	InProgressOperationCount string //
	LegalRetentionCount      string // Retention findings do not by themselves authorize deletion or bypass closure review; legally retained evidence survives ordinary-profile anonymization
	PrecheckContractVersion  string //
	PrecheckedAt             string //
	RetentionPolicyCode      string //
	AnonymizationPolicyCode  string //
	IdempotencyKeyDigest     string //
	RequestFingerprint       string //
	RequestedByUserId        string //
	ConfirmedAt              string //
	RequestedAt              string //
	ReviewedByUserId         string //
	ReviewedAt               string //
	CompletedAt              string //
	CreatedAt                string //
	UpdatedAt                string //
}

// customerClosureRequestColumns holds the columns for the table plugin_idc_core_customer_closure_request.
var customerClosureRequestColumns = CustomerClosureRequestColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	RequestNo:                "request_no",
	CustomerNo:               "customer_no",
	ConsentNo:                "consent_no",
	StatusCode:               "status_code",
	SubjectVersion:           "subject_version",
	UnsettledFundsCount:      "unsettled_funds_count",
	ActiveServiceCount:       "active_service_count",
	UnsettledInvoiceCount:    "unsettled_invoice_count",
	ActiveContractCount:      "active_contract_count",
	InProgressOperationCount: "in_progress_operation_count",
	LegalRetentionCount:      "legal_retention_count",
	PrecheckContractVersion:  "precheck_contract_version",
	PrecheckedAt:             "prechecked_at",
	RetentionPolicyCode:      "retention_policy_code",
	AnonymizationPolicyCode:  "anonymization_policy_code",
	IdempotencyKeyDigest:     "idempotency_key_digest",
	RequestFingerprint:       "request_fingerprint",
	RequestedByUserId:        "requested_by_user_id",
	ConfirmedAt:              "confirmed_at",
	RequestedAt:              "requested_at",
	ReviewedByUserId:         "reviewed_by_user_id",
	ReviewedAt:               "reviewed_at",
	CompletedAt:              "completed_at",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewCustomerClosureRequestDao creates and returns a new DAO object for table data access.
func NewCustomerClosureRequestDao(handlers ...gdb.ModelHandler) *CustomerClosureRequestDao {
	return &CustomerClosureRequestDao{
		group:    "default",
		table:    "plugin_idc_core_customer_closure_request",
		columns:  customerClosureRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerClosureRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerClosureRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerClosureRequestDao) Columns() CustomerClosureRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerClosureRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerClosureRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerClosureRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
