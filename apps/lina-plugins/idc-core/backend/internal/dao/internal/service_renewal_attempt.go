// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceRenewalAttemptDao is the data access object for the table plugin_idc_core_service_renewal_attempt.
type ServiceRenewalAttemptDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  ServiceRenewalAttemptColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// ServiceRenewalAttemptColumns defines and stores column names for the table plugin_idc_core_service_renewal_attempt.
type ServiceRenewalAttemptColumns struct {
	Id                        string //
	TenantId                  string //
	AttemptNo                 string //
	OperationNo               string //
	EntitlementNo             string //
	CustomerNo                string //
	CurrencyCode              string //
	InvoiceNo                 string //
	PolicyNo                  string //
	PolicyVersion             string //
	AttemptSequence           string //
	OutcomeCode               string //
	LedgerTransactionNo       string //
	LedgerTransactionTypeCode string //
	FailureCode               string //
	NextAttemptAt             string //
	EvidenceDigest            string //
	ContentDigest             string //
	CreatedAt                 string //
}

// serviceRenewalAttemptColumns holds the columns for the table plugin_idc_core_service_renewal_attempt.
var serviceRenewalAttemptColumns = ServiceRenewalAttemptColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	AttemptNo:                 "attempt_no",
	OperationNo:               "operation_no",
	EntitlementNo:             "entitlement_no",
	CustomerNo:                "customer_no",
	CurrencyCode:              "currency_code",
	InvoiceNo:                 "invoice_no",
	PolicyNo:                  "policy_no",
	PolicyVersion:             "policy_version",
	AttemptSequence:           "attempt_sequence",
	OutcomeCode:               "outcome_code",
	LedgerTransactionNo:       "ledger_transaction_no",
	LedgerTransactionTypeCode: "ledger_transaction_type_code",
	FailureCode:               "failure_code",
	NextAttemptAt:             "next_attempt_at",
	EvidenceDigest:            "evidence_digest",
	ContentDigest:             "content_digest",
	CreatedAt:                 "created_at",
}

// NewServiceRenewalAttemptDao creates and returns a new DAO object for table data access.
func NewServiceRenewalAttemptDao(handlers ...gdb.ModelHandler) *ServiceRenewalAttemptDao {
	return &ServiceRenewalAttemptDao{
		group:    "default",
		table:    "plugin_idc_core_service_renewal_attempt",
		columns:  serviceRenewalAttemptColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceRenewalAttemptDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceRenewalAttemptDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceRenewalAttemptDao) Columns() ServiceRenewalAttemptColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceRenewalAttemptDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceRenewalAttemptDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceRenewalAttemptDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
