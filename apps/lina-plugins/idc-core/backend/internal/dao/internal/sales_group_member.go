// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesGroupMemberDao is the data access object for the table plugin_idc_core_sales_group_member.
type SalesGroupMemberDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  SalesGroupMemberColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// SalesGroupMemberColumns defines and stores column names for the table plugin_idc_core_sales_group_member.
type SalesGroupMemberColumns struct {
	Id           string //
	TenantId     string //
	MembershipNo string //
	GroupNo      string //
	StaffUserId  string //
	RoleCode     string //
	StatusCode   string //
	EffectiveAt  string //
	EndedAt      string //
	CreatedAt    string //
}

// salesGroupMemberColumns holds the columns for the table plugin_idc_core_sales_group_member.
var salesGroupMemberColumns = SalesGroupMemberColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	MembershipNo: "membership_no",
	GroupNo:      "group_no",
	StaffUserId:  "staff_user_id",
	RoleCode:     "role_code",
	StatusCode:   "status_code",
	EffectiveAt:  "effective_at",
	EndedAt:      "ended_at",
	CreatedAt:    "created_at",
}

// NewSalesGroupMemberDao creates and returns a new DAO object for table data access.
func NewSalesGroupMemberDao(handlers ...gdb.ModelHandler) *SalesGroupMemberDao {
	return &SalesGroupMemberDao{
		group:    "default",
		table:    "plugin_idc_core_sales_group_member",
		columns:  salesGroupMemberColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesGroupMemberDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesGroupMemberDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesGroupMemberDao) Columns() SalesGroupMemberColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesGroupMemberDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesGroupMemberDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesGroupMemberDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
