// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PrivacyDeletionTaskDao is the data access object for the table plugin_idc_compliance_privacy_deletion_task.
type PrivacyDeletionTaskDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  PrivacyDeletionTaskColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// PrivacyDeletionTaskColumns defines and stores column names for the table plugin_idc_compliance_privacy_deletion_task.
type PrivacyDeletionTaskColumns struct {
	Id                string //
	TenantId          string //
	TaskNo            string //
	DataTypeCode      string //
	TargetTypeCode    string //
	TargetNo          string //
	StatusCode        string //
	DueAt             string //
	HoldReasonCode    string //
	ConfirmedByUserId string //
	ConfirmedAt       string //
	CompletedAt       string //
	Version           string //
	CreatedAt         string //
	UpdatedAt         string //
}

// privacyDeletionTaskColumns holds the columns for the table plugin_idc_compliance_privacy_deletion_task.
var privacyDeletionTaskColumns = PrivacyDeletionTaskColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	TaskNo:            "task_no",
	DataTypeCode:      "data_type_code",
	TargetTypeCode:    "target_type_code",
	TargetNo:          "target_no",
	StatusCode:        "status_code",
	DueAt:             "due_at",
	HoldReasonCode:    "hold_reason_code",
	ConfirmedByUserId: "confirmed_by_user_id",
	ConfirmedAt:       "confirmed_at",
	CompletedAt:       "completed_at",
	Version:           "version",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewPrivacyDeletionTaskDao creates and returns a new DAO object for table data access.
func NewPrivacyDeletionTaskDao(handlers ...gdb.ModelHandler) *PrivacyDeletionTaskDao {
	return &PrivacyDeletionTaskDao{
		group:    "default",
		table:    "plugin_idc_compliance_privacy_deletion_task",
		columns:  privacyDeletionTaskColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PrivacyDeletionTaskDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PrivacyDeletionTaskDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PrivacyDeletionTaskDao) Columns() PrivacyDeletionTaskColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PrivacyDeletionTaskDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PrivacyDeletionTaskDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PrivacyDeletionTaskDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
