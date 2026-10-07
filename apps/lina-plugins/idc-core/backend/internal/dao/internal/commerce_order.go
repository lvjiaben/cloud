// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceOrderDao is the data access object for the table plugin_idc_core_commerce_order.
type CommerceOrderDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  CommerceOrderColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// CommerceOrderColumns defines and stores column names for the table plugin_idc_core_commerce_order.
type CommerceOrderColumns struct {
	Id                           string //
	TenantId                     string //
	OrderNo                      string //
	CustomerNo                   string //
	QuoteNo                      string //
	CurrencyCode                 string //
	SourceCode                   string //
	StatusCode                   string //
	RiskDecisionCode             string //
	RequiresContract             string //
	ItemCount                    string //
	SubtotalMinor                string //
	DiscountMinor                string //
	NetAmountMinor               string //
	TaxAmountMinor               string //
	TotalAmountMinor             string //
	CheckoutIdempotencyDigest    string //
	CheckoutRequestDigest        string //
	PaymentDueAt                 string //
	PaidAt                       string //
	CompletedAt                  string //
	CancelledAt                  string //
	Version                      string //
	CreatedBy                    string //
	CreatedAt                    string //
	UpdatedAt                    string //
	AssistedRequestNo            string //
	PaymentTermCode              string //
	ContractOverridePolicyCode   string // Order choice preserved separately: inherit product, explicit none, or required
	ContractOverrideTemplateNo   string //
	EnterpriseContractPolicyCode string //
	EnterpriseContractTemplateNo string //
	ContractDecisionSourceCode   string // Winning rule source after order > product > enterprise > default resolution
	ContractTemplateNo           string //
	ContractTemplateVersionNo    string //
	ContractTemplateName         string //
	ContractLanguageCode         string //
	ContractScopeCode            string //
	ContractSigningOrderCode     string //
}

// commerceOrderColumns holds the columns for the table plugin_idc_core_commerce_order.
var commerceOrderColumns = CommerceOrderColumns{
	Id:                           "id",
	TenantId:                     "tenant_id",
	OrderNo:                      "order_no",
	CustomerNo:                   "customer_no",
	QuoteNo:                      "quote_no",
	CurrencyCode:                 "currency_code",
	SourceCode:                   "source_code",
	StatusCode:                   "status_code",
	RiskDecisionCode:             "risk_decision_code",
	RequiresContract:             "requires_contract",
	ItemCount:                    "item_count",
	SubtotalMinor:                "subtotal_minor",
	DiscountMinor:                "discount_minor",
	NetAmountMinor:               "net_amount_minor",
	TaxAmountMinor:               "tax_amount_minor",
	TotalAmountMinor:             "total_amount_minor",
	CheckoutIdempotencyDigest:    "checkout_idempotency_digest",
	CheckoutRequestDigest:        "checkout_request_digest",
	PaymentDueAt:                 "payment_due_at",
	PaidAt:                       "paid_at",
	CompletedAt:                  "completed_at",
	CancelledAt:                  "cancelled_at",
	Version:                      "version",
	CreatedBy:                    "created_by",
	CreatedAt:                    "created_at",
	UpdatedAt:                    "updated_at",
	AssistedRequestNo:            "assisted_request_no",
	PaymentTermCode:              "payment_term_code",
	ContractOverridePolicyCode:   "contract_override_policy_code",
	ContractOverrideTemplateNo:   "contract_override_template_no",
	EnterpriseContractPolicyCode: "enterprise_contract_policy_code",
	EnterpriseContractTemplateNo: "enterprise_contract_template_no",
	ContractDecisionSourceCode:   "contract_decision_source_code",
	ContractTemplateNo:           "contract_template_no",
	ContractTemplateVersionNo:    "contract_template_version_no",
	ContractTemplateName:         "contract_template_name",
	ContractLanguageCode:         "contract_language_code",
	ContractScopeCode:            "contract_scope_code",
	ContractSigningOrderCode:     "contract_signing_order_code",
}

// NewCommerceOrderDao creates and returns a new DAO object for table data access.
func NewCommerceOrderDao(handlers ...gdb.ModelHandler) *CommerceOrderDao {
	return &CommerceOrderDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_order",
		columns:  commerceOrderColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceOrderDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceOrderDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceOrderDao) Columns() CommerceOrderColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceOrderDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceOrderDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceOrderDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
