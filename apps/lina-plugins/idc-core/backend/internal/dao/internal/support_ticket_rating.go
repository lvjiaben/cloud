// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketRatingDao is the data access object for the table plugin_idc_core_support_ticket_rating.
type SupportTicketRatingDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SupportTicketRatingColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SupportTicketRatingColumns defines and stores column names for the table plugin_idc_core_support_ticket_rating.
type SupportTicketRatingColumns struct {
	Id         string //
	TenantId   string //
	RatingNo   string //
	TicketNo   string //
	RoundNo    string //
	Score      string //
	Comment    string //
	AuthorType string //
	CreatedAt  string //
}

// supportTicketRatingColumns holds the columns for the table plugin_idc_core_support_ticket_rating.
var supportTicketRatingColumns = SupportTicketRatingColumns{
	Id:         "id",
	TenantId:   "tenant_id",
	RatingNo:   "rating_no",
	TicketNo:   "ticket_no",
	RoundNo:    "round_no",
	Score:      "score",
	Comment:    "comment",
	AuthorType: "author_type",
	CreatedAt:  "created_at",
}

// NewSupportTicketRatingDao creates and returns a new DAO object for table data access.
func NewSupportTicketRatingDao(handlers ...gdb.ModelHandler) *SupportTicketRatingDao {
	return &SupportTicketRatingDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_rating",
		columns:  supportTicketRatingColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketRatingDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketRatingDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketRatingDao) Columns() SupportTicketRatingColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketRatingDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketRatingDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketRatingDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
