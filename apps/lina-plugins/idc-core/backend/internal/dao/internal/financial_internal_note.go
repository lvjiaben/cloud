// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// FinancialInternalNoteDao is the data access object for the table plugin_idc_core_financial_internal_note.
type FinancialInternalNoteDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  FinancialInternalNoteColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// FinancialInternalNoteColumns defines and stores column names for the table plugin_idc_core_financial_internal_note.
type FinancialInternalNoteColumns struct {
	Id                string //
	TenantId          string //
	NoteNo            string //
	TargetTypeCode    string //
	TargetNo          string //
	CustomerNo        string //
	Content           string //
	CorrectsNoteNo    string //
	CreatedBy         string //
	IdempotencyDigest string //
	RequestDigest     string //
	CreatedAt         string //
}

// financialInternalNoteColumns holds the columns for the table plugin_idc_core_financial_internal_note.
var financialInternalNoteColumns = FinancialInternalNoteColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	NoteNo:            "note_no",
	TargetTypeCode:    "target_type_code",
	TargetNo:          "target_no",
	CustomerNo:        "customer_no",
	Content:           "content",
	CorrectsNoteNo:    "corrects_note_no",
	CreatedBy:         "created_by",
	IdempotencyDigest: "idempotency_digest",
	RequestDigest:     "request_digest",
	CreatedAt:         "created_at",
}

// NewFinancialInternalNoteDao creates and returns a new DAO object for table data access.
func NewFinancialInternalNoteDao(handlers ...gdb.ModelHandler) *FinancialInternalNoteDao {
	return &FinancialInternalNoteDao{
		group:    "default",
		table:    "plugin_idc_core_financial_internal_note",
		columns:  financialInternalNoteColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *FinancialInternalNoteDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *FinancialInternalNoteDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *FinancialInternalNoteDao) Columns() FinancialInternalNoteColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *FinancialInternalNoteDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *FinancialInternalNoteDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *FinancialInternalNoteDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
