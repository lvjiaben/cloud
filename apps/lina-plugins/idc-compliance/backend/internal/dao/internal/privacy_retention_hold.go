// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyRetentionHoldDao is the data access object for the table plugin_idc_compliance_privacy_retention_hold.
type PrivacyRetentionHoldDao struct {
	table    string                      // table is the underlying table name of the DAO.
	group    string                      // group is the database configuration group name of the current DAO.
	columns  PrivacyRetentionHoldColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler          // handlers for customized model modification.
}

// PrivacyRetentionHoldColumns defines and stores column names for the table plugin_idc_compliance_privacy_retention_hold.
type PrivacyRetentionHoldColumns struct {
	Id              string //
	TenantId        string //
	HoldNo          string //
	TargetTypeCode  string //
	TargetNo        string //
	ReasonCode      string //
	StatusCode      string //
	CreatedByOwner  string //
	ReleasedByOwner string //
	ReleasedAt      string //
	CreatedAt       string //
	UpdatedAt       string //
}

// privacyRetentionHoldColumns holds the columns for the table plugin_idc_compliance_privacy_retention_hold.
var privacyRetentionHoldColumns = PrivacyRetentionHoldColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	HoldNo:          "hold_no",
	TargetTypeCode:  "target_type_code",
	TargetNo:        "target_no",
	ReasonCode:      "reason_code",
	StatusCode:      "status_code",
	CreatedByOwner:  "created_by_owner",
	ReleasedByOwner: "released_by_owner",
	ReleasedAt:      "released_at",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewPrivacyRetentionHoldDao creates and returns a new DAO object for table data access.
func NewPrivacyRetentionHoldDao(handlers ...gdb.ModelHandler) *PrivacyRetentionHoldDao {
	return &PrivacyRetentionHoldDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_retention_hold",
		columns:  privacyRetentionHoldColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyRetentionHoldDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyRetentionHoldDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyRetentionHoldDao) Columns() PrivacyRetentionHoldColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyRetentionHoldDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyRetentionHoldDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacyRetentionHoldDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
