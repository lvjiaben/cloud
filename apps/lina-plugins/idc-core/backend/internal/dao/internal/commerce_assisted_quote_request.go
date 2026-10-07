// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceAssistedQuoteRequestDao is the data access object for the table plugin_idc_core_commerce_assisted_quote_request.
type CommerceAssistedQuoteRequestDao struct {
	table    string                              // table is the underlying table name of the DAO.
	group    string                              // group is the database configuration group name of the current DAO.
	columns  CommerceAssistedQuoteRequestColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                  // handlers for customized model modification.
}

// CommerceAssistedQuoteRequestColumns defines and stores column names for the table plugin_idc_core_commerce_assisted_quote_request.
type CommerceAssistedQuoteRequestColumns struct {
	Id                         string //
	TenantId                   string //
	RequestNo                  string //
	CustomerNo                 string //
	CurrencyCode               string //
	StatusCode                 string //
	ItemCount                  string //
	ManualDiscountMinor        string //
	PaymentTermCode            string //
	PaymentDueSeconds          string //
	Reason                     string //
	InternalNote               string //
	IdempotencyDigest          string //
	RequestDigest              string //
	ExpiresAt                  string //
	RequestedBy                string //
	DecisionCode               string //
	DecisionBy                 string //
	DecisionAt                 string //
	DecisionReason             string //
	DecisionRequestDigest      string //
	DecisionEvidenceDigest     string //
	QuoteNo                    string //
	OrderNo                    string //
	Version                    string //
	CreatedAt                  string //
	UpdatedAt                  string //
	ContractOverridePolicyCode string // Administrator order choice captured before assisted checkout
	ContractOverrideTemplateNo string //
}

// commerceAssistedQuoteRequestColumns holds the columns for the table plugin_idc_core_commerce_assisted_quote_request.
var commerceAssistedQuoteRequestColumns = CommerceAssistedQuoteRequestColumns{
	Id:                         "id",
	TenantId:                   "tenant_id",
	RequestNo:                  "request_no",
	CustomerNo:                 "customer_no",
	CurrencyCode:               "currency_code",
	StatusCode:                 "status_code",
	ItemCount:                  "item_count",
	ManualDiscountMinor:        "manual_discount_minor",
	PaymentTermCode:            "payment_term_code",
	PaymentDueSeconds:          "payment_due_seconds",
	Reason:                     "reason",
	InternalNote:               "internal_note",
	IdempotencyDigest:          "idempotency_digest",
	RequestDigest:              "request_digest",
	ExpiresAt:                  "expires_at",
	RequestedBy:                "requested_by",
	DecisionCode:               "decision_code",
	DecisionBy:                 "decision_by",
	DecisionAt:                 "decision_at",
	DecisionReason:             "decision_reason",
	DecisionRequestDigest:      "decision_request_digest",
	DecisionEvidenceDigest:     "decision_evidence_digest",
	QuoteNo:                    "quote_no",
	OrderNo:                    "order_no",
	Version:                    "version",
	CreatedAt:                  "created_at",
	UpdatedAt:                  "updated_at",
	ContractOverridePolicyCode: "contract_override_policy_code",
	ContractOverrideTemplateNo: "contract_override_template_no",
}

// NewCommerceAssistedQuoteRequestDao creates and returns a new DAO object for table data access.
func NewCommerceAssistedQuoteRequestDao(handlers ...gdb.ModelHandler) *CommerceAssistedQuoteRequestDao {
	return &CommerceAssistedQuoteRequestDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_assisted_quote_request",
		columns:  commerceAssistedQuoteRequestColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceAssistedQuoteRequestDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceAssistedQuoteRequestDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceAssistedQuoteRequestDao) Columns() CommerceAssistedQuoteRequestColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceAssistedQuoteRequestDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceAssistedQuoteRequestDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceAssistedQuoteRequestDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
