// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAgentApplicationDao is the data access object for the table plugin_idc_core_sales_agent_application.
type SalesAgentApplicationDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SalesAgentApplicationColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SalesAgentApplicationColumns defines and stores column names for the table plugin_idc_core_sales_agent_application.
type SalesAgentApplicationColumns struct {
	Id                string //
	TenantId          string //
	ApplicationNo     string //
	CustomerNo        string //
	RequestedLevelNo  string //
	StatusCode        string //
	ApplicationDigest string //
	SubmittedAt       string //
	ReviewedBy        string //
	ReviewReasonCode  string //
	ReviewedAt        string //
	Version           string //
	CreatedAt         string //
	UpdatedAt         string //
}

// salesAgentApplicationColumns holds the columns for the table plugin_idc_core_sales_agent_application.
var salesAgentApplicationColumns = SalesAgentApplicationColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	ApplicationNo:     "application_no",
	CustomerNo:        "customer_no",
	RequestedLevelNo:  "requested_level_no",
	StatusCode:        "status_code",
	ApplicationDigest: "application_digest",
	SubmittedAt:       "submitted_at",
	ReviewedBy:        "reviewed_by",
	ReviewReasonCode:  "review_reason_code",
	ReviewedAt:        "reviewed_at",
	Version:           "version",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewSalesAgentApplicationDao creates and returns a new DAO object for table data access.
func NewSalesAgentApplicationDao(handlers ...gdb.ModelHandler) *SalesAgentApplicationDao {
	return &SalesAgentApplicationDao{
		group:    "default",
		table:    "plugin_idc_core_sales_agent_application",
		columns:  salesAgentApplicationColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAgentApplicationDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAgentApplicationDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAgentApplicationDao) Columns() SalesAgentApplicationColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAgentApplicationDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAgentApplicationDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAgentApplicationDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
