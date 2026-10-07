// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAffiliateTouchDao is the data access object for the table plugin_idc_core_sales_affiliate_touch.
type SalesAffiliateTouchDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  SalesAffiliateTouchColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// SalesAffiliateTouchColumns defines and stores column names for the table plugin_idc_core_sales_affiliate_touch.
type SalesAffiliateTouchColumns struct {
	Id             string //
	TenantId       string //
	TouchNo        string //
	LinkNo         string //
	VisitorDigest  string //
	DeviceDigest   string //
	IpDigest       string //
	EvidenceDigest string //
	OccurredAt     string //
	ExpiresAt      string //
}

// salesAffiliateTouchColumns holds the columns for the table plugin_idc_core_sales_affiliate_touch.
var salesAffiliateTouchColumns = SalesAffiliateTouchColumns{
	Id:             "id",
	TenantId:       "tenant_id",
	TouchNo:        "touch_no",
	LinkNo:         "link_no",
	VisitorDigest:  "visitor_digest",
	DeviceDigest:   "device_digest",
	IpDigest:       "ip_digest",
	EvidenceDigest: "evidence_digest",
	OccurredAt:     "occurred_at",
	ExpiresAt:      "expires_at",
}

// NewSalesAffiliateTouchDao creates and returns a new DAO object for table data access.
func NewSalesAffiliateTouchDao(handlers ...gdb.ModelHandler) *SalesAffiliateTouchDao {
	return &SalesAffiliateTouchDao{
		group:    "default",
		table:    "plugin_idc_core_sales_affiliate_touch",
		columns:  salesAffiliateTouchColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAffiliateTouchDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAffiliateTouchDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAffiliateTouchDao) Columns() SalesAffiliateTouchColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAffiliateTouchDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAffiliateTouchDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAffiliateTouchDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
