// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TrialEntitlementDao is the data access object for the table plugin_idc_core_trial_entitlement.
type TrialEntitlementDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  TrialEntitlementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// TrialEntitlementColumns defines and stores column names for the table plugin_idc_core_trial_entitlement.
type TrialEntitlementColumns struct {
	Id                     string //
	TenantId               string //
	EntitlementNo          string //
	ScopeNo                string //
	CustomerNo             string //
	ProductNo              string //
	VersionNo              string //
	BillingRuleNo          string //
	OrderNo                string //
	InvoiceNo              string //
	OrderItemNo            string //
	CurrencyCode           string //
	TrialLimitQuantity     string //
	StatusCode             string //
	HoldExpiresAt          string //
	ConsumedAt             string //
	ReleasedAt             string //
	IdentityEvidenceDigest string //
	ContentDigest          string //
	Version                string //
	CreatedAt              string //
	UpdatedAt              string //
	UnitSequence           string // Generic one-based quantity unit inside the immutable order item; it has no server meaning.
}

// trialEntitlementColumns holds the columns for the table plugin_idc_core_trial_entitlement.
var trialEntitlementColumns = TrialEntitlementColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	EntitlementNo:          "entitlement_no",
	ScopeNo:                "scope_no",
	CustomerNo:             "customer_no",
	ProductNo:              "product_no",
	VersionNo:              "version_no",
	BillingRuleNo:          "billing_rule_no",
	OrderNo:                "order_no",
	InvoiceNo:              "invoice_no",
	OrderItemNo:            "order_item_no",
	CurrencyCode:           "currency_code",
	TrialLimitQuantity:     "trial_limit_quantity",
	StatusCode:             "status_code",
	HoldExpiresAt:          "hold_expires_at",
	ConsumedAt:             "consumed_at",
	ReleasedAt:             "released_at",
	IdentityEvidenceDigest: "identity_evidence_digest",
	ContentDigest:          "content_digest",
	Version:                "version",
	CreatedAt:              "created_at",
	UpdatedAt:              "updated_at",
	UnitSequence:           "unit_sequence",
}

// NewTrialEntitlementDao creates and returns a new DAO object for table data access.
func NewTrialEntitlementDao(handlers ...gdb.ModelHandler) *TrialEntitlementDao {
	return &TrialEntitlementDao{
		group:    "default",
		table:    "plugin_idc_core_trial_entitlement",
		columns:  trialEntitlementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TrialEntitlementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TrialEntitlementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TrialEntitlementDao) Columns() TrialEntitlementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TrialEntitlementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TrialEntitlementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TrialEntitlementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
