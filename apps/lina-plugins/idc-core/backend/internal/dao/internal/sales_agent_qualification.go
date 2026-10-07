// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentQualificationDao is the data access object for the table plugin_idc_core_sales_agent_qualification.
type SalesAgentQualificationDao struct {
	table    string                         // table is the underlying table name of the DAO.
	group    string                         // group is the database configuration group name of the current DAO.
	columns  SalesAgentQualificationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler             // handlers for customized model modification.
}

// SalesAgentQualificationColumns defines and stores column names for the table plugin_idc_core_sales_agent_qualification.
type SalesAgentQualificationColumns struct {
	Id                  string //
	TenantId            string //
	QualificationNo     string //
	CustomerNo          string //
	LevelNo             string //
	SourceApplicationNo string //
	StatusCode          string //
	EffectiveAt         string //
	ExpiresAt           string //
	Version             string //
	CreatedAt           string //
	UpdatedAt           string //
}

// salesAgentQualificationColumns holds the columns for the table plugin_idc_core_sales_agent_qualification.
var salesAgentQualificationColumns = SalesAgentQualificationColumns{
	Id:                  "id",
	TenantId:            "tenant_id",
	QualificationNo:     "qualification_no",
	CustomerNo:          "customer_no",
	LevelNo:             "level_no",
	SourceApplicationNo: "source_application_no",
	StatusCode:          "status_code",
	EffectiveAt:         "effective_at",
	ExpiresAt:           "expires_at",
	Version:             "version",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewSalesAgentQualificationDao creates and returns a new DAO object for table data access.
func NewSalesAgentQualificationDao(handlers ...gdb.ModelHandler) *SalesAgentQualificationDao {
	return &SalesAgentQualificationDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_qualification",
		columns:  salesAgentQualificationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentQualificationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentQualificationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentQualificationDao) Columns() SalesAgentQualificationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentQualificationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentQualificationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentQualificationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
