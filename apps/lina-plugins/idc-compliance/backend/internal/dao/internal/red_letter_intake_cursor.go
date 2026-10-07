// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// RedLetterIntakeCursorDao is the data access object for the table plugin_idc_compliance_red_letter_intake_cursor.
type RedLetterIntakeCursorDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  RedLetterIntakeCursorColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// RedLetterIntakeCursorColumns defines and stores column names for the table plugin_idc_compliance_red_letter_intake_cursor.
type RedLetterIntakeCursorColumns struct {
	Id           string //
	TenantId     string //
	FeedCode     string //
	SourceCursor string //
	Version      string //
	CreatedAt    string //
	UpdatedAt    string //
}

// redLetterIntakeCursorColumns holds the columns for the table plugin_idc_compliance_red_letter_intake_cursor.
var redLetterIntakeCursorColumns = RedLetterIntakeCursorColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	FeedCode:     "feed_code",
	SourceCursor: "source_cursor",
	Version:      "version",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewRedLetterIntakeCursorDao creates and returns a new DAO object for table data access.
func NewRedLetterIntakeCursorDao(handlers ...gdb.ModelHandler) *RedLetterIntakeCursorDao {
	return &RedLetterIntakeCursorDao{
		group:    "default",
		table:    "plugin_idc_compliance_red_letter_intake_cursor",
		columns:  redLetterIntakeCursorColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *RedLetterIntakeCursorDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *RedLetterIntakeCursorDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *RedLetterIntakeCursorDao) Columns() RedLetterIntakeCursorColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *RedLetterIntakeCursorDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *RedLetterIntakeCursorDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *RedLetterIntakeCursorDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
