// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CommerceAssistedQuoteRequestItemDao is the data access object for the table plugin_idc_core_commerce_assisted_quote_request_item.
type CommerceAssistedQuoteRequestItemDao struct {
	table    string                                  // table is the underlying table name of the DAO.
	group    string                                  // group is the database configuration group name of the current DAO.
	columns  CommerceAssistedQuoteRequestItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                      // handlers for customized model modification.
}

// CommerceAssistedQuoteRequestItemColumns defines and stores column names for the table plugin_idc_core_commerce_assisted_quote_request_item.
type CommerceAssistedQuoteRequestItemColumns struct {
	Id                      string //
	TenantId                string //
	RequestItemNo           string //
	RequestNo               string //
	CurrencyCode            string //
	LineNumber              string //
	ProductNo               string //
	VersionNo               string //
	BillingRuleNo           string //
	ConfigurationSnapshotNo string //
	ProductNameZhCn         string //
	ProductNameEnUs         string //
	PurchaseCycles          string //
	ContentDigest           string //
	CreatedAt               string //
}

// commerceAssistedQuoteRequestItemColumns holds the columns for the table plugin_idc_core_commerce_assisted_quote_request_item.
var commerceAssistedQuoteRequestItemColumns = CommerceAssistedQuoteRequestItemColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	RequestItemNo:           "request_item_no",
	RequestNo:               "request_no",
	CurrencyCode:            "currency_code",
	LineNumber:              "line_number",
	ProductNo:               "product_no",
	VersionNo:               "version_no",
	BillingRuleNo:           "billing_rule_no",
	ConfigurationSnapshotNo: "configuration_snapshot_no",
	ProductNameZhCn:         "product_name_zh_cn",
	ProductNameEnUs:         "product_name_en_us",
	PurchaseCycles:          "purchase_cycles",
	ContentDigest:           "content_digest",
	CreatedAt:               "created_at",
}

// NewCommerceAssistedQuoteRequestItemDao creates and returns a new DAO object for table data access.
func NewCommerceAssistedQuoteRequestItemDao(handlers ...gdb.ModelHandler) *CommerceAssistedQuoteRequestItemDao {
	return &CommerceAssistedQuoteRequestItemDao{
		group:    "default",
		table:    "plugin_idc_core_commerce_assisted_quote_request_item",
		columns:  commerceAssistedQuoteRequestItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CommerceAssistedQuoteRequestItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CommerceAssistedQuoteRequestItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CommerceAssistedQuoteRequestItemDao) Columns() CommerceAssistedQuoteRequestItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CommerceAssistedQuoteRequestItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CommerceAssistedQuoteRequestItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CommerceAssistedQuoteRequestItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
