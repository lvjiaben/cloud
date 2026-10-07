// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingValueEntryDao is the data access object for the table plugin_idc_core_marketing_value_entry.
type MarketingValueEntryDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  MarketingValueEntryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// MarketingValueEntryColumns defines and stores column names for the table plugin_idc_core_marketing_value_entry.
type MarketingValueEntryColumns struct {
	Id                  string //
	TenantId            string //
	EntryNo             string //
	GrantNo             string //
	CustomerNo          string //
	CurrencyCode        string //
	EntryTypeCode       string //
	PendingDeltaMinor   string //
	AvailableDeltaMinor string //
	FrozenDeltaMinor    string //
	ConsumedDeltaMinor  string //
	ExpiredDeltaMinor   string //
	ReversedDeltaMinor  string //
	SourceTypeCode      string //
	SourceReference     string //
	ReversalOfEntryNo   string //
	CreatedAt           string //
}

// marketingValueEntryColumns holds the columns for the table plugin_idc_core_marketing_value_entry.
var marketingValueEntryColumns = MarketingValueEntryColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	EntryNo:             "entry_no",
	GrantNo:             "grant_no",
	CustomerNo:          "customer_no",
	CurrencyCode:        "currency_code",
	EntryTypeCode:       "entry_type_code",
	PendingDeltaMinor:   "pending_delta_minor",
	AvailableDeltaMinor: "available_delta_minor",
	FrozenDeltaMinor:    "frozen_delta_minor",
	ConsumedDeltaMinor:  "consumed_delta_minor",
	ExpiredDeltaMinor:   "expired_delta_minor",
	ReversedDeltaMinor:  "reversed_delta_minor",
	SourceTypeCode:      "source_type_code",
	SourceReference:     "source_reference",
	ReversalOfEntryNo:   "reversal_of_entry_no",
	CreatedAt:           "created_at",
}

// NewMarketingValueEntryDao creates and returns a new DAO object for table data access.
func NewMarketingValueEntryDao(handlers ...gdb.ModelHandler) *MarketingValueEntryDao {
	return &MarketingValueEntryDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_value_entry",
		columns:  marketingValueEntryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingValueEntryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingValueEntryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingValueEntryDao) Columns() MarketingValueEntryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingValueEntryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingValueEntryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingValueEntryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
