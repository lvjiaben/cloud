// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// CustomerRiskRecordDao is the data access object for the table plugin_idc_core_customer_risk_record.
type CustomerRiskRecordDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  CustomerRiskRecordColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// CustomerRiskRecordColumns defines and stores column names for the table plugin_idc_core_customer_risk_record.
type CustomerRiskRecordColumns struct {
	Id                string //
	TenantId          string //
	RiskNo            string //
	CustomerNo        string //
	CategoryCode      string //
	SourceCode        string //
	LevelCode         string //
	ScoreBasisPoints  string //
	StatusCode        string //
	Summary           string //
	EvidenceReference string //
	OccurredAt        string //
	AcknowledgedBy    string //
	AcknowledgedAt    string //
	ResolvedBy        string //
	ResolvedAt        string //
	Resolution        string //
	CreatedAt         string //
	UpdatedAt         string //
}

// customerRiskRecordColumns holds the columns for the table plugin_idc_core_customer_risk_record.
var customerRiskRecordColumns = CustomerRiskRecordColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	RiskNo:            "risk_no",
	CustomerNo:        "customer_no",
	CategoryCode:      "category_code",
	SourceCode:        "source_code",
	LevelCode:         "level_code",
	ScoreBasisPoints:  "score_basis_points",
	StatusCode:        "status_code",
	Summary:           "summary",
	EvidenceReference: "evidence_reference",
	OccurredAt:        "occurred_at",
	AcknowledgedBy:    "acknowledged_by",
	AcknowledgedAt:    "acknowledged_at",
	ResolvedBy:        "resolved_by",
	ResolvedAt:        "resolved_at",
	Resolution:        "resolution",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewCustomerRiskRecordDao creates and returns a new DAO object for table data access.
func NewCustomerRiskRecordDao(handlers ...gdb.ModelHandler) *CustomerRiskRecordDao {
	return &CustomerRiskRecordDao{
		group:    "default",
		table:    "plugin_idc_core_customer_risk_record",
		columns:  customerRiskRecordColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *CustomerRiskRecordDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *CustomerRiskRecordDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *CustomerRiskRecordDao) Columns() CustomerRiskRecordColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *CustomerRiskRecordDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *CustomerRiskRecordDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *CustomerRiskRecordDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
