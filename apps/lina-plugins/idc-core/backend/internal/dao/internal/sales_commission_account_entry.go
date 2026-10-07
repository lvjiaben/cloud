// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionAccountEntryDao is the data access object for the table plugin_idc_core_sales_commission_account_entry.
type SalesCommissionAccountEntryDao struct {
	table    string                             // table is the underlying table name of the DAO.
	group    string                             // group is the database configuration group name of the current DAO.
	columns  SalesCommissionAccountEntryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                 // handlers for customized model modification.
}

// SalesCommissionAccountEntryColumns defines and stores column names for the table plugin_idc_core_sales_commission_account_entry.
type SalesCommissionAccountEntryColumns struct {
	Id                  string //
	TenantId            string //
	EntryNo             string //
	AccountNo           string //
	CurrencyCode        string //
	EntrySequence       string //
	EntryTypeCode       string //
	SourceTypeCode      string //
	SourceReference     string //
	SourceDigest        string //
	DeltaCreditedMinor  string //
	DeltaPendingMinor   string //
	DeltaAvailableMinor string //
	DeltaFrozenMinor    string //
	DeltaPaidMinor      string //
	DeltaReversedMinor  string //
	AfterCreditedMinor  string //
	AfterPendingMinor   string //
	AfterAvailableMinor string //
	AfterFrozenMinor    string //
	AfterPaidMinor      string //
	AfterReversedMinor  string //
	OccurredAt          string //
}

// salesCommissionAccountEntryColumns holds the columns for the table plugin_idc_core_sales_commission_account_entry.
var salesCommissionAccountEntryColumns = SalesCommissionAccountEntryColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	EntryNo:             "entry_no",
	AccountNo:           "account_no",
	CurrencyCode:        "currency_code",
	EntrySequence:       "entry_sequence",
	EntryTypeCode:       "entry_type_code",
	SourceTypeCode:      "source_type_code",
	SourceReference:     "source_reference",
	SourceDigest:        "source_digest",
	DeltaCreditedMinor:  "delta_credited_minor",
	DeltaPendingMinor:   "delta_pending_minor",
	DeltaAvailableMinor: "delta_available_minor",
	DeltaFrozenMinor:    "delta_frozen_minor",
	DeltaPaidMinor:      "delta_paid_minor",
	DeltaReversedMinor:  "delta_reversed_minor",
	AfterCreditedMinor:  "after_credited_minor",
	AfterPendingMinor:   "after_pending_minor",
	AfterAvailableMinor: "after_available_minor",
	AfterFrozenMinor:    "after_frozen_minor",
	AfterPaidMinor:      "after_paid_minor",
	AfterReversedMinor:  "after_reversed_minor",
	OccurredAt:          "occurred_at",
}

// NewSalesCommissionAccountEntryDao creates and returns a new DAO object for table data access.
func NewSalesCommissionAccountEntryDao(handlers ...gdb.ModelHandler) *SalesCommissionAccountEntryDao {
	return &SalesCommissionAccountEntryDao{
		group:    "default",
		table:    "plugin_idc_core_sales_commission_account_entry",
		columns:  salesCommissionAccountEntryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesCommissionAccountEntryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesCommissionAccountEntryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesCommissionAccountEntryDao) Columns() SalesCommissionAccountEntryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesCommissionAccountEntryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesCommissionAccountEntryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesCommissionAccountEntryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
