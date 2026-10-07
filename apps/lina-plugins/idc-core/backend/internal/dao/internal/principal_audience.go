// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrincipalAudienceDao is the data access object for the table plugin_idc_core_principal_audience.
type PrincipalAudienceDao struct {
	table    string                   // table is the underlying table name of the DAO.
	group    string                   // group is the database configuration group name of the current DAO.
	columns  PrincipalAudienceColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler       // handlers for customized model modification.
}

// PrincipalAudienceColumns defines and stores column names for the table plugin_idc_core_principal_audience.
type PrincipalAudienceColumns struct {
	Id           string //
	TenantId     string //
	SysUserId    string // Host sys_user identity ID validated through host capabilities
	AudienceCode string // Audience code: customer or staff
	StatusCode   string // Binding status code: active or revoked
	BoundAt      string // Time when the audience binding became active
	RevokedAt    string // Time when the audience binding was revoked
	Version      string //
	CreatedBy    string //
	UpdatedBy    string //
	CreatedAt    string //
	UpdatedAt    string //
}

// principalAudienceColumns holds the columns for the table plugin_idc_core_principal_audience.
var principalAudienceColumns = PrincipalAudienceColumns{
	Id:           "id",
	TenantId:     "tenant_id",
	SysUserId:    "sys_user_id",
	AudienceCode: "audience_code",
	StatusCode:   "status_code",
	BoundAt:      "bound_at",
	RevokedAt:    "revoked_at",
	Version:      "version",
	CreatedBy:    "created_by",
	UpdatedBy:    "updated_by",
	CreatedAt:    "created_at",
	UpdatedAt:    "updated_at",
}

// NewPrincipalAudienceDao creates and returns a new DAO object for table data access.
func NewPrincipalAudienceDao(handlers ...gdb.ModelHandler) *PrincipalAudienceDao {
	return &PrincipalAudienceDao{
		group:    "default",
		table:    "plugin_idc_core_principal_audience",
		columns:  principalAudienceColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrincipalAudienceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrincipalAudienceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrincipalAudienceDao) Columns() PrincipalAudienceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrincipalAudienceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrincipalAudienceDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrincipalAudienceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
