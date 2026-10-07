// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryQueryDao is the data access object for the table plugin_idc_notify_delivery_query.
type DeliveryQueryDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  DeliveryQueryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// DeliveryQueryColumns defines and stores column names for the table plugin_idc_notify_delivery_query.
type DeliveryQueryColumns struct {
	TenantId       string //
	QueryNo        string //
	TaskNo         string //
	SourceCode     string //
	ResultClass    string //
	EvidenceDigest string //
	ObservedAt     string //
}

// deliveryQueryColumns holds the columns for the table plugin_idc_notify_delivery_query.
var deliveryQueryColumns = DeliveryQueryColumns{
	TenantId:       "tenant_id",
	QueryNo:        "query_no",
	TaskNo:         "task_no",
	SourceCode:     "source_code",
	ResultClass:    "result_class",
	EvidenceDigest: "evidence_digest",
	ObservedAt:     "observed_at",
}

// NewDeliveryQueryDao creates and returns a new DAO object for table data access.
func NewDeliveryQueryDao(handlers ...gdb.ModelHandler) *DeliveryQueryDao {
	return &DeliveryQueryDao{
		group:    "default",
		table:    "plugin_idc_notify_delivery_query",
		columns:  deliveryQueryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *DeliveryQueryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *DeliveryQueryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *DeliveryQueryDao) Columns() DeliveryQueryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *DeliveryQueryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *DeliveryQueryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *DeliveryQueryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
