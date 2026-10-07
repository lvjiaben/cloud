// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SalesAffiliateAttributionDao is the data access object for the table plugin_idc_core_sales_affiliate_attribution.
type SalesAffiliateAttributionDao struct {
	table    string                           // table is the underlying table name of the DAO.
	group    string                           // group is the database configuration group name of the current DAO.
	columns  SalesAffiliateAttributionColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler               // handlers for customized model modification.
}

// SalesAffiliateAttributionColumns defines and stores column names for the table plugin_idc_core_sales_affiliate_attribution.
type SalesAffiliateAttributionColumns struct {
	Id                 string //
	TenantId           string //
	AttributionNo      string //
	LinkNo             string //
	TouchNo            string //
	ReferrerCustomerNo string //
	ReferredCustomerNo string //
	PolicyCode         string //
	StatusCode         string //
	RiskReasonCode     string //
	EvidenceDigest     string //
	AttributedAt       string //
	ExpiresAt          string //
}

// salesAffiliateAttributionColumns holds the columns for the table plugin_idc_core_sales_affiliate_attribution.
var salesAffiliateAttributionColumns = SalesAffiliateAttributionColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	AttributionNo:      "attribution_no",
	LinkNo:             "link_no",
	TouchNo:            "touch_no",
	ReferrerCustomerNo: "referrer_customer_no",
	ReferredCustomerNo: "referred_customer_no",
	PolicyCode:         "policy_code",
	StatusCode:         "status_code",
	RiskReasonCode:     "risk_reason_code",
	EvidenceDigest:     "evidence_digest",
	AttributedAt:       "attributed_at",
	ExpiresAt:          "expires_at",
}

// NewSalesAffiliateAttributionDao creates and returns a new DAO object for table data access.
func NewSalesAffiliateAttributionDao(handlers ...gdb.ModelHandler) *SalesAffiliateAttributionDao {
	return &SalesAffiliateAttributionDao{
		group:    "default",
		table:    "plugin_idc_core_sales_affiliate_attribution",
		columns:  salesAffiliateAttributionColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SalesAffiliateAttributionDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SalesAffiliateAttributionDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SalesAffiliateAttributionDao) Columns() SalesAffiliateAttributionColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SalesAffiliateAttributionDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SalesAffiliateAttributionDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SalesAffiliateAttributionDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
