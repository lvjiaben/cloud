// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAffiliateLinkDao is the data access object for the table plugin_idc_core_sales_affiliate_link.
type SalesAffiliateLinkDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  SalesAffiliateLinkColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// SalesAffiliateLinkColumns defines and stores column names for the table plugin_idc_core_sales_affiliate_link.
type SalesAffiliateLinkColumns struct {
	Id                       string //
	TenantId                 string //
	LinkNo                   string //
	OwnerCustomerNo          string //
	CodeDigest               string //
	AttributionPolicyCode    string //
	AttributionWindowSeconds string //
	StatusCode               string //
	ExpiresAt                string //
	Version                  string //
	CreatedAt                string //
	UpdatedAt                string //
}

// salesAffiliateLinkColumns holds the columns for the table plugin_idc_core_sales_affiliate_link.
var salesAffiliateLinkColumns = SalesAffiliateLinkColumns{
	Id:                       "id",
	TenantId:                 "tenant_id",
	LinkNo:                   "link_no",
	OwnerCustomerNo:          "owner_customer_no",
	CodeDigest:               "code_digest",
	AttributionPolicyCode:    "attribution_policy_code",
	AttributionWindowSeconds: "attribution_window_seconds",
	StatusCode:               "status_code",
	ExpiresAt:                "expires_at",
	Version:                  "version",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
}

// NewSalesAffiliateLinkDao creates and returns a new DAO object for table data access.
func NewSalesAffiliateLinkDao(handlers ...gdb.ModelHandler) *SalesAffiliateLinkDao {
	return &SalesAffiliateLinkDao{
		group:    "default",
		table:    "plugin_idc_core_sales_affiliate_link",
		columns:  salesAffiliateLinkColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAffiliateLinkDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAffiliateLinkDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAffiliateLinkDao) Columns() SalesAffiliateLinkColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAffiliateLinkDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAffiliateLinkDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAffiliateLinkDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
