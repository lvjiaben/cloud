// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportTicketRelationDao is the data access object for the table plugin_idc_core_support_ticket_relation.
type SupportTicketRelationDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SupportTicketRelationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SupportTicketRelationColumns defines and stores column names for the table plugin_idc_core_support_ticket_relation.
type SupportTicketRelationColumns struct {
	Id             string //
	TenantId       string //
	RelationNo     string //
	RelationType   string //
	SourceTicketNo string //
	TargetTicketNo string //
	ActorUserId    string //
	Reason         string //
	CreatedAt      string //
}

// supportTicketRelationColumns holds the columns for the table plugin_idc_core_support_ticket_relation.
var supportTicketRelationColumns = SupportTicketRelationColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	RelationNo:     "relation_no",
	RelationType:   "relation_type",
	SourceTicketNo: "source_ticket_no",
	TargetTicketNo: "target_ticket_no",
	ActorUserId:    "actor_user_id",
	Reason:         "reason",
	CreatedAt:      "created_at",
}

// NewSupportTicketRelationDao creates and returns a new DAO object for table data access.
func NewSupportTicketRelationDao(handlers ...gdb.ModelHandler) *SupportTicketRelationDao {
	return &SupportTicketRelationDao{
		group:    "default",
		table:    "plugin_idc_core_support_ticket_relation",
		columns:  supportTicketRelationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportTicketRelationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportTicketRelationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportTicketRelationDao) Columns() SupportTicketRelationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportTicketRelationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportTicketRelationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportTicketRelationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
