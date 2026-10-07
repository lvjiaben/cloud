// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingPointsEntryDao is the data access object for the table plugin_idc_core_marketing_points_entry.
type MarketingPointsEntryDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  MarketingPointsEntryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// MarketingPointsEntryColumns defines and stores column names for the table plugin_idc_core_marketing_points_entry.
type MarketingPointsEntryColumns struct {
	Id                string //
	TenantId          string //
	EntryNo           string //
	CustomerNo        string //
	LotNo             string //
	EntryTypeCode     string //
	AvailableDelta    string //
	FrozenDelta       string //
	DebtDelta         string //
	SourceTypeCode    string //
	SourceReference   string //
	EntryLineNo       string //
	ReversalOfEntryNo string //
	CreatedAt         string //
}

// marketingPointsEntryColumns holds the columns for the table plugin_idc_core_marketing_points_entry.
var marketingPointsEntryColumns = MarketingPointsEntryColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	EntryNo:           "entry_no",
	CustomerNo:        "customer_no",
	LotNo:             "lot_no",
	EntryTypeCode:     "entry_type_code",
	AvailableDelta:    "available_delta",
	FrozenDelta:       "frozen_delta",
	DebtDelta:         "debt_delta",
	SourceTypeCode:    "source_type_code",
	SourceReference:   "source_reference",
	EntryLineNo:       "entry_line_no",
	ReversalOfEntryNo: "reversal_of_entry_no",
	CreatedAt:         "created_at",
}

// NewMarketingPointsEntryDao creates and returns a new DAO object for table data access.
func NewMarketingPointsEntryDao(handlers ...gdb.ModelHandler) *MarketingPointsEntryDao {
	return &MarketingPointsEntryDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_points_entry",
		columns:  marketingPointsEntryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingPointsEntryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingPointsEntryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingPointsEntryDao) Columns() MarketingPointsEntryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingPointsEntryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingPointsEntryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingPointsEntryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
