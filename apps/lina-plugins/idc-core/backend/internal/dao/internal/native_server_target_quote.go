// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NativeServerTargetQuoteDao is the data access object for the table plugin_idc_core_native_server_target_quote.
type NativeServerTargetQuoteDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  NativeServerTargetQuoteColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// NativeServerTargetQuoteColumns defines and stores column names for the table plugin_idc_core_native_server_target_quote.
type NativeServerTargetQuoteColumns struct {
	TenantId                   string //
	Id                         string //
	TargetQuoteNo              string //
	QuoteItemNo                string //
	StructuredConfigurationNo  string //
	EntitlementNo              string //
	CustomerNo                 string //
	ExpectedEntitlementVersion string //
	CustomerSubjectVersion     string //
	CurrencyCode               string //
	NativeTargetSnapshot       string //
	CreatedAt                  string //
}

// nativeServerTargetQuoteColumns holds the columns for the table plugin_idc_core_native_server_target_quote.
var nativeServerTargetQuoteColumns = NativeServerTargetQuoteColumns{
	TenantId:                   "tenant_id",
	Id:                         "id",
	TargetQuoteNo:              "target_quote_no",
	QuoteItemNo:                "quote_item_no",
	StructuredConfigurationNo:  "structured_configuration_no",
	EntitlementNo:              "entitlement_no",
	CustomerNo:                 "customer_no",
	ExpectedEntitlementVersion: "expected_entitlement_version",
	CustomerSubjectVersion:     "customer_subject_version",
	CurrencyCode:               "currency_code",
	NativeTargetSnapshot:       "native_target_snapshot",
	CreatedAt:                  "created_at",
}

// NewNativeServerTargetQuoteDao creates and returns a new DAO object for table data access.
func NewNativeServerTargetQuoteDao(handlers ...gdb.ModelHandler) *NativeServerTargetQuoteDao {
	return &NativeServerTargetQuoteDao{
		group:    "default",
		table:    "plugin_idc_core_native_server_target_quote",
		columns:  nativeServerTargetQuoteColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NativeServerTargetQuoteDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NativeServerTargetQuoteDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NativeServerTargetQuoteDao) Columns() NativeServerTargetQuoteColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NativeServerTargetQuoteDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NativeServerTargetQuoteDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *NativeServerTargetQuoteDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
