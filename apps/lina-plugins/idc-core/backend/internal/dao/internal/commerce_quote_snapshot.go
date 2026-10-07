// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceQuoteSnapshotDao is the data access object for the table plugin_idc_core_commerce_quote_snapshot.
type CommerceQuoteSnapshotDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  CommerceQuoteSnapshotColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// CommerceQuoteSnapshotColumns defines and stores column names for the table plugin_idc_core_commerce_quote_snapshot.
type CommerceQuoteSnapshotColumns struct {
	Id                     string //
	TenantId               string //
	QuoteNo                string //
	CartNo                 string //
	CustomerNo             string //
	CustomerSubjectVersion string //
	PurposeCode            string //
	SourceCode             string //
	CurrencyCode           string //
	ItemCount              string //
	SubtotalMinor          string //
	DiscountMinor          string //
	NetAmountMinor         string //
	TaxAmountMinor         string //
	TotalAmountMinor       string //
	InputDigest            string //
	PricingRuleDigest      string //
	EligibilityDigest      string //
	InventoryDigest        string //
	ContentDigest          string //
	IssuedAt               string //
	ExpiresAt              string //
	CreatedBy              string //
}

// commerceQuoteSnapshotColumns holds the columns for the table plugin_idc_core_commerce_quote_snapshot.
var commerceQuoteSnapshotColumns = CommerceQuoteSnapshotColumns{
	Id:                     "id",
	TenantId:               "tenant_id",
	QuoteNo:                "quote_no",
	CartNo:                 "cart_no",
	CustomerNo:             "customer_no",
	CustomerSubjectVersion: "customer_subject_version",
	PurposeCode:            "purpose_code",
	SourceCode:             "source_code",
	CurrencyCode:           "currency_code",
	ItemCount:              "item_count",
	SubtotalMinor:          "subtotal_minor",
	DiscountMinor:          "discount_minor",
	NetAmountMinor:         "net_amount_minor",
	TaxAmountMinor:         "tax_amount_minor",
	TotalAmountMinor:       "total_amount_minor",
	InputDigest:            "input_digest",
	PricingRuleDigest:      "pricing_rule_digest",
	EligibilityDigest:      "eligibility_digest",
	InventoryDigest:        "inventory_digest",
	ContentDigest:          "content_digest",
	IssuedAt:               "issued_at",
	ExpiresAt:              "expires_at",
	CreatedBy:              "created_by",
}

// NewCommerceQuoteSnapshotDao creates and returns a new DAO object for table data access.
func NewCommerceQuoteSnapshotDao(handlers ...gdb.ModelHandler) *CommerceQuoteSnapshotDao {
	return &CommerceQuoteSnapshotDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_quote_snapshot",
		columns:  commerceQuoteSnapshotColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceQuoteSnapshotDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceQuoteSnapshotDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceQuoteSnapshotDao) Columns() CommerceQuoteSnapshotColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceQuoteSnapshotDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceQuoteSnapshotDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceQuoteSnapshotDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
