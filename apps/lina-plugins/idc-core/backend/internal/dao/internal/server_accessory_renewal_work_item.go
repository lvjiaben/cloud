// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryRenewalWorkItemDao is the data access object for the table plugin_idc_core_server_accessory_renewal_work_item.
type ServerAccessoryRenewalWorkItemDao struct {
	table    string                                // table is the underlying table name of the DAO.
	group    string                                // group is the database configuration group name of the current DAO.
	columns  ServerAccessoryRenewalWorkItemColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler                    // handlers for customized model modification.
}

// ServerAccessoryRenewalWorkItemColumns defines and stores column names for the table plugin_idc_core_server_accessory_renewal_work_item.
type ServerAccessoryRenewalWorkItemColumns struct {
	Id                 string //
	TenantId           string //
	WorkItemNo         string //
	EntitlementNo      string //
	CustomerNo         string //
	ServiceNo          string //
	InvoiceNo          string //
	RequiredPermission string //
	StatusCode         string //
	ReasonCode         string //
	CreatedAt          string //
	ResolvedAt         string //
	UpdatedAt          string //
	SubjectTypeCode    string //
	SubjectNo          string //
}

// serverAccessoryRenewalWorkItemColumns holds the columns for the table plugin_idc_core_server_accessory_renewal_work_item.
var serverAccessoryRenewalWorkItemColumns = ServerAccessoryRenewalWorkItemColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	WorkItemNo:         "work_item_no",
	EntitlementNo:      "entitlement_no",
	CustomerNo:         "customer_no",
	ServiceNo:          "service_no",
	InvoiceNo:          "invoice_no",
	RequiredPermission: "required_permission",
	StatusCode:         "status_code",
	ReasonCode:         "reason_code",
	CreatedAt:          "created_at",
	ResolvedAt:         "resolved_at",
	UpdatedAt:          "updated_at",
	SubjectTypeCode:    "subject_type_code",
	SubjectNo:          "subject_no",
}

// NewServerAccessoryRenewalWorkItemDao creates and returns a new DAO object for table data access.
func NewServerAccessoryRenewalWorkItemDao(handlers ...gdb.ModelHandler) *ServerAccessoryRenewalWorkItemDao {
	return &ServerAccessoryRenewalWorkItemDao{
		group:    "default",
		table:    "plugin_idc_core_server_accessory_renewal_work_item",
		columns:  serverAccessoryRenewalWorkItemColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ServerAccessoryRenewalWorkItemDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ServerAccessoryRenewalWorkItemDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ServerAccessoryRenewalWorkItemDao) Columns() ServerAccessoryRenewalWorkItemColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ServerAccessoryRenewalWorkItemDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ServerAccessoryRenewalWorkItemDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ServerAccessoryRenewalWorkItemDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
