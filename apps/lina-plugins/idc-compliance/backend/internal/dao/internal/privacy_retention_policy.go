// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyRetentionPolicyDao is the data access object for the table plugin_idc_compliance_privacy_retention_policy.
type PrivacyRetentionPolicyDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  PrivacyRetentionPolicyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// PrivacyRetentionPolicyColumns defines and stores column names for the table plugin_idc_compliance_privacy_retention_policy.
type PrivacyRetentionPolicyColumns struct {
	Id              string //
	TenantId        string //
	PolicyNo        string //
	DataTypeCode    string //
	Version         string //
	RetentionDays   string //
	StatusCode      string //
	CreatedByUserId string //
	SupersededAt    string //
	CreatedAt       string //
}

// privacyRetentionPolicyColumns holds the columns for the table plugin_idc_compliance_privacy_retention_policy.
var privacyRetentionPolicyColumns = PrivacyRetentionPolicyColumns{
	Id:              "id",
	TenantId:        "tenant_id",
	PolicyNo:        "policy_no",
	DataTypeCode:    "data_type_code",
	Version:         "version",
	RetentionDays:   "retention_days",
	StatusCode:      "status_code",
	CreatedByUserId: "created_by_user_id",
	SupersededAt:    "superseded_at",
	CreatedAt:       "created_at",
}

// NewPrivacyRetentionPolicyDao creates and returns a new DAO object for table data access.
func NewPrivacyRetentionPolicyDao(handlers ...gdb.ModelHandler) *PrivacyRetentionPolicyDao {
	return &PrivacyRetentionPolicyDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_retention_policy",
		columns:  privacyRetentionPolicyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyRetentionPolicyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyRetentionPolicyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyRetentionPolicyDao) Columns() PrivacyRetentionPolicyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyRetentionPolicyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyRetentionPolicyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacyRetentionPolicyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
