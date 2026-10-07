// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BillingAdjustmentRequestDao is the data access object for the table plugin_idc_core_billing_adjustment_request.
type BillingAdjustmentRequestDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  BillingAdjustmentRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// BillingAdjustmentRequestColumns defines and stores column names for the table plugin_idc_core_billing_adjustment_request.
type BillingAdjustmentRequestColumns struct {
	Id                        string //
	TenantId                  string //
	AdjustmentNo              string //
	CustomerNo                string //
	InvoiceNo                 string //
	AccountNo                 string //
	AccountTypeCode           string //
	CurrencyCode              string //
	AdjustmentTypeCode        string //
	AmountMinor               string //
	ReasonCode                string //
	EvidenceDigest            string //
	IdempotencyDigest         string //
	RequestDigest             string //
	ApprovalRequired          string //
	StatusCode                string //
	RequestedBy               string //
	LedgerTransactionNo       string //
	LedgerTransactionTypeCode string //
	PostedAt                  string //
	Version                   string //
	CreatedAt                 string //
	UpdatedAt                 string //
	ProofReference            string // Verified receipt evidence or the exact marketing value-grant business number
	ReceivingAccountReference string //
	BusinessPurposeCode       string //
	PolicyNo                  string //
	PolicyVersion             string //
	CreditStatementNo         string //
}

// billingAdjustmentRequestColumns holds the columns for the table plugin_idc_core_billing_adjustment_request.
var billingAdjustmentRequestColumns = BillingAdjustmentRequestColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	AdjustmentNo:              "adjustment_no",
	CustomerNo:                "customer_no",
	InvoiceNo:                 "invoice_no",
	AccountNo:                 "account_no",
	AccountTypeCode:           "account_type_code",
	CurrencyCode:              "currency_code",
	AdjustmentTypeCode:        "adjustment_type_code",
	AmountMinor:               "amount_minor",
	ReasonCode:                "reason_code",
	EvidenceDigest:            "evidence_digest",
	IdempotencyDigest:         "idempotency_digest",
	RequestDigest:             "request_digest",
	ApprovalRequired:          "approval_required",
	StatusCode:                "status_code",
	RequestedBy:               "requested_by",
	LedgerTransactionNo:       "ledger_transaction_no",
	LedgerTransactionTypeCode: "ledger_transaction_type_code",
	PostedAt:                  "posted_at",
	Version:                   "version",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
	ProofReference:            "proof_reference",
	ReceivingAccountReference: "receiving_account_reference",
	BusinessPurposeCode:       "business_purpose_code",
	PolicyNo:                  "policy_no",
	PolicyVersion:             "policy_version",
	CreditStatementNo:         "credit_statement_no",
}

// NewBillingAdjustmentRequestDao creates and returns a new DAO object for table data access.
func NewBillingAdjustmentRequestDao(handlers ...gdb.ModelHandler) *BillingAdjustmentRequestDao {
	return &BillingAdjustmentRequestDao{
		group:    "default",
		table:    "plugin_idc_core_billing_adjustment_request",
		columns:  billingAdjustmentRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BillingAdjustmentRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BillingAdjustmentRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BillingAdjustmentRequestDao) Columns() BillingAdjustmentRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BillingAdjustmentRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BillingAdjustmentRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BillingAdjustmentRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
