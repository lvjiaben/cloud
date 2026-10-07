// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketingValueGrantDao is the data access object for the table plugin_idc_core_marketing_value_grant.
type MarketingValueGrantDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  MarketingValueGrantColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// MarketingValueGrantColumns defines and stores column names for the table plugin_idc_core_marketing_value_grant.
type MarketingValueGrantColumns struct {
	Id                        string //
	TenantId                  string //
	GrantNo                   string //
	GrantTypeCode             string //
	CampaignNo                string //
	CampaignTypeCode          string //
	BenefitNo                 string //
	BenefitTypeCode           string //
	BenefitSnapshotDigest     string //
	CustomerNo                string //
	CurrencyCode              string //
	SourceReference           string //
	OriginalAmountMinor       string //
	PendingAmountMinor        string //
	AvailableAmountMinor      string //
	FrozenAmountMinor         string //
	ConsumedAmountMinor       string //
	ExpiredAmountMinor        string //
	ReversedAmountMinor       string //
	Withdrawable              string //
	GlobalQuotaReserved       string //
	CustomerQuotaReserved     string //
	ExpiresAt                 string //
	StatusCode                string //
	FinancialPostingReference string //
	CreatedAt                 string //
	UpdatedAt                 string //
}

// marketingValueGrantColumns holds the columns for the table plugin_idc_core_marketing_value_grant.
var marketingValueGrantColumns = MarketingValueGrantColumns{
	Id:                        "id",
	TenantId:                  "tenant_id",
	GrantNo:                   "grant_no",
	GrantTypeCode:             "grant_type_code",
	CampaignNo:                "campaign_no",
	CampaignTypeCode:          "campaign_type_code",
	BenefitNo:                 "benefit_no",
	BenefitTypeCode:           "benefit_type_code",
	BenefitSnapshotDigest:     "benefit_snapshot_digest",
	CustomerNo:                "customer_no",
	CurrencyCode:              "currency_code",
	SourceReference:           "source_reference",
	OriginalAmountMinor:       "original_amount_minor",
	PendingAmountMinor:        "pending_amount_minor",
	AvailableAmountMinor:      "available_amount_minor",
	FrozenAmountMinor:         "frozen_amount_minor",
	ConsumedAmountMinor:       "consumed_amount_minor",
	ExpiredAmountMinor:        "expired_amount_minor",
	ReversedAmountMinor:       "reversed_amount_minor",
	Withdrawable:              "withdrawable",
	GlobalQuotaReserved:       "global_quota_reserved",
	CustomerQuotaReserved:     "customer_quota_reserved",
	ExpiresAt:                 "expires_at",
	StatusCode:                "status_code",
	FinancialPostingReference: "financial_posting_reference",
	CreatedAt:                 "created_at",
	UpdatedAt:                 "updated_at",
}

// NewMarketingValueGrantDao creates and returns a new DAO object for table data access.
func NewMarketingValueGrantDao(handlers ...gdb.ModelHandler) *MarketingValueGrantDao {
	return &MarketingValueGrantDao{
		group:    "default",
		table:    "plugin_idc_core_marketing_value_grant",
		columns:  marketingValueGrantColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *MarketingValueGrantDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *MarketingValueGrantDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *MarketingValueGrantDao) Columns() MarketingValueGrantColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *MarketingValueGrantDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *MarketingValueGrantDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *MarketingValueGrantDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
