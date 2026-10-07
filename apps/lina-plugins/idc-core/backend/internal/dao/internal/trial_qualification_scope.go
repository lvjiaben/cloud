// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TrialQualificationScopeDao is the data access object for the table plugin_idc_core_trial_qualification_scope.
type TrialQualificationScopeDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  TrialQualificationScopeColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// TrialQualificationScopeColumns defines and stores column names for the table plugin_idc_core_trial_qualification_scope.
type TrialQualificationScopeColumns struct {
	Id               string //
	TenantId         string //
	ScopeNo          string //
	CustomerNo       string //
	ProductNo        string //
	HeldQuantity     string //
	ConsumedQuantity string //
	Version          string //
	CreatedAt        string //
	UpdatedAt        string //
}

// trialQualificationScopeColumns holds the columns for the table plugin_idc_core_trial_qualification_scope.
var trialQualificationScopeColumns = TrialQualificationScopeColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	ScopeNo:          "scope_no",
	CustomerNo:       "customer_no",
	ProductNo:        "product_no",
	HeldQuantity:     "held_quantity",
	ConsumedQuantity: "consumed_quantity",
	Version:          "version",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewTrialQualificationScopeDao creates and returns a new DAO object for table data access.
func NewTrialQualificationScopeDao(handlers ...gdb.ModelHandler) *TrialQualificationScopeDao {
	return &TrialQualificationScopeDao{
		group:    "default",
		table:    "plugin_idc_core_trial_qualification_scope",
		columns:  trialQualificationScopeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TrialQualificationScopeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TrialQualificationScopeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TrialQualificationScopeDao) Columns() TrialQualificationScopeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TrialQualificationScopeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TrialQualificationScopeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TrialQualificationScopeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
