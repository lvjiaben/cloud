// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TrialQualificationMovementDao is the data access object for the table plugin_idc_core_trial_qualification_movement.
type TrialQualificationMovementDao struct {
	table    string                            // table is the underlying table name of the DAO.
	group    string                            // group is the database configuration group name of the current DAO.
	columns  TrialQualificationMovementColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                // handlers for customized model modification.
}

// TrialQualificationMovementColumns defines and stores column names for the table plugin_idc_core_trial_qualification_movement.
type TrialQualificationMovementColumns struct {
	Id                 string //
	TenantId           string //
	MovementNo         string //
	ScopeNo            string //
	CustomerNo         string //
	ProductNo          string //
	OrderNo            string //
	MovementTypeCode   string //
	Quantity           string //
	HeldBefore         string //
	HeldAfter          string //
	ConsumedBefore     string //
	ConsumedAfter      string //
	ScopeVersionBefore string //
	ScopeVersionAfter  string //
	EvidenceDigest     string //
	ContentDigest      string //
	CreatedAt          string //
}

// trialQualificationMovementColumns holds the columns for the table plugin_idc_core_trial_qualification_movement.
var trialQualificationMovementColumns = TrialQualificationMovementColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	MovementNo:         "movement_no",
	ScopeNo:            "scope_no",
	CustomerNo:         "customer_no",
	ProductNo:          "product_no",
	OrderNo:            "order_no",
	MovementTypeCode:   "movement_type_code",
	Quantity:           "quantity",
	HeldBefore:         "held_before",
	HeldAfter:          "held_after",
	ConsumedBefore:     "consumed_before",
	ConsumedAfter:      "consumed_after",
	ScopeVersionBefore: "scope_version_before",
	ScopeVersionAfter:  "scope_version_after",
	EvidenceDigest:     "evidence_digest",
	ContentDigest:      "content_digest",
	CreatedAt:          "created_at",
}

// NewTrialQualificationMovementDao creates and returns a new DAO object for table data access.
func NewTrialQualificationMovementDao(handlers ...gdb.ModelHandler) *TrialQualificationMovementDao {
	return &TrialQualificationMovementDao{
		group:    "default",
		table:    "plugin_idc_core_trial_qualification_movement",
		columns:  trialQualificationMovementColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TrialQualificationMovementDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TrialQualificationMovementDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TrialQualificationMovementDao) Columns() TrialQualificationMovementColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TrialQualificationMovementDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TrialQualificationMovementDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TrialQualificationMovementDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
