// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerNoteDao is the data access object for the table plugin_idc_core_customer_note.
type CustomerNoteDao struct {
	table    string              // table is the underlying table name of the DAO.
	group    string              // group is the database configuration group name of the current DAO.
	columns  CustomerNoteColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler  // handlers for customized model modification.
}

// CustomerNoteColumns defines and stores column names for the table plugin_idc_core_customer_note.
type CustomerNoteColumns struct {
	Id               string //
	TenantId         string //
	NoteNo           string //
	CustomerNo       string //
	CategoryCode     string //
	Content          string //
	StatusCode       string //
	SupersedesNoteNo string //
	AuthorUserId     string //
	WithdrawnBy      string //
	WithdrawnAt      string //
	CreatedAt        string //
	UpdatedAt        string //
}

// customerNoteColumns holds the columns for the table plugin_idc_core_customer_note.
var customerNoteColumns = CustomerNoteColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	NoteNo:           "note_no",
	CustomerNo:       "customer_no",
	CategoryCode:     "category_code",
	Content:          "content",
	StatusCode:       "status_code",
	SupersedesNoteNo: "supersedes_note_no",
	AuthorUserId:     "author_user_id",
	WithdrawnBy:      "withdrawn_by",
	WithdrawnAt:      "withdrawn_at",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewCustomerNoteDao creates and returns a new DAO object for table data access.
func NewCustomerNoteDao(handlers ...gdb.ModelHandler) *CustomerNoteDao {
	return &CustomerNoteDao{
		group:    "default",
		table:    "plugin_idc_core_customer_note",
		columns:  customerNoteColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerNoteDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerNoteDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerNoteDao) Columns() CustomerNoteColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerNoteDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerNoteDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerNoteDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
