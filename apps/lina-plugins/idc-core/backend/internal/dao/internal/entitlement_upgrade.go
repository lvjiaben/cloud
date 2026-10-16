// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// EntitlementUpgradeDao is the data access object for the table plugin_idc_core_entitlement_upgrade.
type EntitlementUpgradeDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  EntitlementUpgradeColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// EntitlementUpgradeColumns defines and stores column names for the table plugin_idc_core_entitlement_upgrade.
type EntitlementUpgradeColumns struct {
	Id                    string //
	TenantId              string //
	UpgradeNo             string //
	CustomerNo            string //
	EntitlementNo         string //
	ProductNo             string //
	SourceVersion         string //
	SourceConfigurationNo string //
	TargetConfigurationNo string //
	TargetPriceSnapshotNo string //
	CurrencyCode          string //
	SourcePeriodNetMinor  string //
	TargetPeriodNetMinor  string //
	ChargeNetMinor        string //
	ChargeTaxMinor        string //
	ChargeTotalMinor      string //
	PeriodStartedAt       string //
	PeriodExpiresAt       string //
	QuoteExpiresAt        string //
	FrozenTarget          string //
	FrozenPrice           string //
	ProviderParameters    string //
	ChannelNo             string //
	PoolNo                string //
	InvoiceNo             string //
	ProviderTaskNo        string //
	StatusCode            string //
	IdempotencyDigest     string //
	RequestDigest         string //
	ContentDigest         string //
	LastErrorCode         string //
	AttemptCount          string //
	AvailableAt           string //
	PaidAt                string //
	CompletedAt           string //
	Version               string //
	CreatedAt             string //
	UpdatedAt             string //
}

// entitlementUpgradeColumns holds the columns for the table plugin_idc_core_entitlement_upgrade.
var entitlementUpgradeColumns = EntitlementUpgradeColumns{
	Id:                    "id",
	TenantId:              "tenant_id",
	UpgradeNo:             "upgrade_no",
	CustomerNo:            "customer_no",
	EntitlementNo:         "entitlement_no",
	ProductNo:             "product_no",
	SourceVersion:         "source_version",
	SourceConfigurationNo: "source_configuration_no",
	TargetConfigurationNo: "target_configuration_no",
	TargetPriceSnapshotNo: "target_price_snapshot_no",
	CurrencyCode:          "currency_code",
	SourcePeriodNetMinor:  "source_period_net_minor",
	TargetPeriodNetMinor:  "target_period_net_minor",
	ChargeNetMinor:        "charge_net_minor",
	ChargeTaxMinor:        "charge_tax_minor",
	ChargeTotalMinor:      "charge_total_minor",
	PeriodStartedAt:       "period_started_at",
	PeriodExpiresAt:       "period_expires_at",
	QuoteExpiresAt:        "quote_expires_at",
	FrozenTarget:          "frozen_target",
	FrozenPrice:           "frozen_price",
	ProviderParameters:    "provider_parameters",
	ChannelNo:             "channel_no",
	PoolNo:                "pool_no",
	InvoiceNo:             "invoice_no",
	ProviderTaskNo:        "provider_task_no",
	StatusCode:            "status_code",
	IdempotencyDigest:     "idempotency_digest",
	RequestDigest:         "request_digest",
	ContentDigest:         "content_digest",
	LastErrorCode:         "last_error_code",
	AttemptCount:          "attempt_count",
	AvailableAt:           "available_at",
	PaidAt:                "paid_at",
	CompletedAt:           "completed_at",
	Version:               "version",
	CreatedAt:             "created_at",
	UpdatedAt:             "updated_at",
}

// NewEntitlementUpgradeDao creates and returns a new DAO object for table data access.
func NewEntitlementUpgradeDao(handlers ...gdb.ModelHandler) *EntitlementUpgradeDao {
	return &EntitlementUpgradeDao{
		group:    "default",
		table:    "plugin_idc_core_entitlement_upgrade",
		columns:  entitlementUpgradeColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *EntitlementUpgradeDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *EntitlementUpgradeDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *EntitlementUpgradeDao) Columns() EntitlementUpgradeColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *EntitlementUpgradeDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *EntitlementUpgradeDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *EntitlementUpgradeDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
