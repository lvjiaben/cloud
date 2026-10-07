// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegmentConditionDao is the data access object for the table plugin_idc_core_marketing_segment_condition.
type MarketingSegmentConditionDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  MarketingSegmentConditionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// MarketingSegmentConditionColumns defines and stores column names for the table plugin_idc_core_marketing_segment_condition.
type MarketingSegmentConditionColumns struct {
	Id                string //
	TenantId          string //
	ConditionNo       string //
	SegmentNo         string //
	OrGroupNo         string //
	SortOrder         string //
	FactCode          string //
	OperatorCode      string //
	ValueKindCode     string //
	ValueText         string //
	ValueIntegerLower string //
	ValueIntegerUpper string //
	ValueBoolean      string //
	CurrencyCode      string //
	CreatedAt         string //
}

// marketingSegmentConditionColumns holds the columns for the table plugin_idc_core_marketing_segment_condition.
var marketingSegmentConditionColumns = MarketingSegmentConditionColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	ConditionNo:       "condition_no",
	SegmentNo:         "segment_no",
	OrGroupNo:         "or_group_no",
	SortOrder:         "sort_order",
	FactCode:          "fact_code",
	OperatorCode:      "operator_code",
	ValueKindCode:     "value_kind_code",
	ValueText:         "value_text",
	ValueIntegerLower: "value_integer_lower",
	ValueIntegerUpper: "value_integer_upper",
	ValueBoolean:      "value_boolean",
	CurrencyCode:      "currency_code",
	CreatedAt:         "created_at",
}

// NewMarketingSegmentConditionDao creates and returns a new DAO object for table data access.
func NewMarketingSegmentConditionDao(handlers ...gdb.ModelHandler) *MarketingSegmentConditionDao {
	return &MarketingSegmentConditionDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_segment_condition",
		columns:  marketingSegmentConditionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingSegmentConditionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingSegmentConditionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingSegmentConditionDao) Columns() MarketingSegmentConditionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingSegmentConditionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingSegmentConditionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingSegmentConditionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
