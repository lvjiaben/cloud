// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TaxInvoiceActionDao is the data access object for the table plugin_idc_compliance_tax_invoice_action.
type TaxInvoiceActionDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  TaxInvoiceActionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// TaxInvoiceActionColumns defines and stores column names for the table plugin_idc_compliance_tax_invoice_action.
type TaxInvoiceActionColumns struct {
	Id                   string //
	TenantId             string //
	ActionNo             string //
	TaxInvoiceNo         string //
	ActionCode           string //
	IdempotencyDigest    string //
	RequestFingerprint   string //
	ActorUserId          string //
	BeforeStatusCode     string //
	AfterStatusCode      string //
	BeforeVersion        string //
	AfterVersion         string //
	InvoiceTypeCode      string //
	ProviderCode         string //
	ReasonCode           string //
	InternalNote         string //
	CapabilityReasonCode string //
	CreatedAt            string //
}

// taxInvoiceActionColumns holds the columns for the table plugin_idc_compliance_tax_invoice_action.
var taxInvoiceActionColumns = TaxInvoiceActionColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	ActionNo:             "action_no",
	TaxInvoiceNo:         "tax_invoice_no",
	ActionCode:           "action_code",
	IdempotencyDigest:    "idempotency_digest",
	RequestFingerprint:   "request_fingerprint",
	ActorUserId:          "actor_user_id",
	BeforeStatusCode:     "before_status_code",
	AfterStatusCode:      "after_status_code",
	BeforeVersion:        "before_version",
	AfterVersion:         "after_version",
	InvoiceTypeCode:      "invoice_type_code",
	ProviderCode:         "provider_code",
	ReasonCode:           "reason_code",
	InternalNote:         "internal_note",
	CapabilityReasonCode: "capability_reason_code",
	CreatedAt:            "created_at",
}

// NewTaxInvoiceActionDao creates and returns a new DAO object for table data access.
func NewTaxInvoiceActionDao(handlers ...gdb.ModelHandler) *TaxInvoiceActionDao {
	return &TaxInvoiceActionDao{
		group:    "default",
		table:    "plugin_idc_compliance_tax_invoice_action",
		columns:  taxInvoiceActionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TaxInvoiceActionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TaxInvoiceActionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TaxInvoiceActionDao) Columns() TaxInvoiceActionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TaxInvoiceActionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TaxInvoiceActionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TaxInvoiceActionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
