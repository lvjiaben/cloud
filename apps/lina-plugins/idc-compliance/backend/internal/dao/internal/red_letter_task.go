// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterTaskDao is the data access object for the table plugin_idc_compliance_red_letter_task.
type RedLetterTaskDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  RedLetterTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// RedLetterTaskColumns defines and stores column names for the table plugin_idc_compliance_red_letter_task.
type RedLetterTaskColumns struct {
	Id                           string //
	TenantId                     string //
	TaskNo                       string //
	TaxInvoiceNo                 string //
	RequestSha256                string //
	RefundOperationNo            string //
	ReasonCode                   string //
	StatusCode                   string //
	GrossAmountMinor             string //
	NetAmountMinor               string //
	TaxAmountMinor               string //
	CustomerConfirmationRequired string //
	CustomerConfirmedAt          string //
	ProviderCode                 string //
	ExternalTaskRef              string //
	AvailableAt                  string //
	NextQueryAt                  string //
	CompletedAt                  string //
	FailureCode                  string //
	Version                      string //
	CreatedAt                    string //
	UpdatedAt                    string //
	CustomerNo                   string //
	CurrencyCode                 string //
	AllocationResolved           string //
	ManualReasonCode             string //
	CompletionEvidenceNo         string //
	CustomerRecheckRequestedAt   string //
}

// redLetterTaskColumns holds the columns for the table plugin_idc_compliance_red_letter_task.
var redLetterTaskColumns = RedLetterTaskColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	TaskNo:                       "task_no",
	TaxInvoiceNo:                 "tax_invoice_no",
	RequestSha256:                "request_sha256",
	RefundOperationNo:            "refund_operation_no",
	ReasonCode:                   "reason_code",
	StatusCode:                   "status_code",
	GrossAmountMinor:             "gross_amount_minor",
	NetAmountMinor:               "net_amount_minor",
	TaxAmountMinor:               "tax_amount_minor",
	CustomerConfirmationRequired: "customer_confirmation_required",
	CustomerConfirmedAt:          "customer_confirmed_at",
	ProviderCode:                 "provider_code",
	ExternalTaskRef:              "external_task_ref",
	AvailableAt:                  "available_at",
	NextQueryAt:                  "next_query_at",
	CompletedAt:                  "completed_at",
	FailureCode:                  "failure_code",
	Version:                      "version",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
	CustomerNo:                   "customer_no",
	CurrencyCode:                 "currency_code",
	AllocationResolved:           "allocation_resolved",
	ManualReasonCode:             "manual_reason_code",
	CompletionEvidenceNo:         "completion_evidence_no",
	CustomerRecheckRequestedAt:   "customer_recheck_requested_at",
}

// NewRedLetterTaskDao creates and returns a new DAO object for table data access.
func NewRedLetterTaskDao(handlers ...gdb.ModelHandler) *RedLetterTaskDao {
	return &RedLetterTaskDao{
		group:    "default",
		table:    "plugin_idc_compliance_red_letter_task",
		columns:  redLetterTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RedLetterTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RedLetterTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RedLetterTaskDao) Columns() RedLetterTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RedLetterTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RedLetterTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RedLetterTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
