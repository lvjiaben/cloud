// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceTrafficSuspensionDao is the data access object for the table plugin_idc_core_service_traffic_suspension.
type ServiceTrafficSuspensionDao struct {
	table    string                          // table is the underlying table name of the DAO.
	group    string                          // group is the database configuration group name of the current DAO.
	columns  ServiceTrafficSuspensionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler              // handlers for customized model modification.
}

// ServiceTrafficSuspensionColumns defines and stores column names for the table plugin_idc_core_service_traffic_suspension.
type ServiceTrafficSuspensionColumns struct {
	Id                string //
	TenantId          string //
	ServiceNo         string //
	SourceOperationNo string //
	Active            string //
	SuspendedAt       string //
	ClearedAt         string //
	Version           string //
	CreatedAt         string //
	UpdatedAt         string //
}

// serviceTrafficSuspensionColumns holds the columns for the table plugin_idc_core_service_traffic_suspension.
var serviceTrafficSuspensionColumns = ServiceTrafficSuspensionColumns{
	Id:                "id",
	TenantId:          "tenant_id",
	ServiceNo:         "service_no",
	SourceOperationNo: "source_operation_no",
	Active:            "active",
	SuspendedAt:       "suspended_at",
	ClearedAt:         "cleared_at",
	Version:           "version",
	CreatedAt:         "created_at",
	UpdatedAt:         "updated_at",
}

// NewServiceTrafficSuspensionDao creates and returns a new DAO object for table data access.
func NewServiceTrafficSuspensionDao(handlers ...gdb.ModelHandler) *ServiceTrafficSuspensionDao {
	return &ServiceTrafficSuspensionDao{
		group:    "default",
		table:    "plugin_idc_core_service_traffic_suspension",
		columns:  serviceTrafficSuspensionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServiceTrafficSuspensionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServiceTrafficSuspensionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServiceTrafficSuspensionDao) Columns() ServiceTrafficSuspensionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServiceTrafficSuspensionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServiceTrafficSuspensionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServiceTrafficSuspensionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
