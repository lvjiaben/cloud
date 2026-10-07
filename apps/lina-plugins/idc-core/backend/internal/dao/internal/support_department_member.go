// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SupportDepartmentMemberDao is the data access object for the table plugin_idc_core_support_department_member.
type SupportDepartmentMemberDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SupportDepartmentMemberColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SupportDepartmentMemberColumns defines and stores column names for the table plugin_idc_core_support_department_member.
type SupportDepartmentMemberColumns struct {
	Id           string //
	TenantId     string //
	DepartmentNo string //
	UserId       string //
	RoleCode     string //
	StatusCode   string //
	Version      string //
	CreatedAt    string //
	UpdatedAt    string //
}

// supportDepartmentMemberColumns holds the columns for the table plugin_idc_core_support_department_member.
var supportDepartmentMemberColumns = SupportDepartmentMemberColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	DepartmentNo: "department_no",
	UserId:       "user_id",
	RoleCode:     "role_code",
	StatusCode:   "status_code",
	Version:      "version",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewSupportDepartmentMemberDao creates and returns a new DAO object for table data access.
func NewSupportDepartmentMemberDao(handlers ...gdb.ModelHandler) *SupportDepartmentMemberDao {
	return &SupportDepartmentMemberDao{
		group:    "default",
		table:    "plugin_idc_core_support_department_member",
		columns:  supportDepartmentMemberColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SupportDepartmentMemberDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SupportDepartmentMemberDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SupportDepartmentMemberDao) Columns() SupportDepartmentMemberColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SupportDepartmentMemberDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SupportDepartmentMemberDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SupportDepartmentMemberDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
