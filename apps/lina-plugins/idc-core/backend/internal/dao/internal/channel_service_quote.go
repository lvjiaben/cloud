// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChannelServiceQuoteDao is the data access object for the table plugin_idc_core_channel_service_quote.
type ChannelServiceQuoteDao struct {
	table    string                     // table is the underlying table name of the DAO.
	group    string                     // group is the database configuration group name of the current DAO.
	columns  ChannelServiceQuoteColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler         // handlers for customized model modification.
}

// ChannelServiceQuoteColumns defines and stores column names for the table plugin_idc_core_channel_service_quote.
type ChannelServiceQuoteColumns struct {
	Id                             string //
	TenantId                       string //
	QuoteNo                        string //
	ChannelId                      string //
	PayerCustomerNo                string //
	ServiceOwnerCustomerNo         string //
	ServiceNo                      string //
	ActionCode                     string //
	CurrencyCode                   string //
	DirectionCode                  string //
	CurrentConfigurationSnapshotNo string //
	TargetConfigurationSnapshotNo  string //
	TargetVersionNo                string //
	TargetBillingRuleNo            string //
	CurrentRecurringAmountMinor    string //
	TargetRecurringAmountMinor     string //
	TargetSubtotalMinor            string //
	TargetNetAmountMinor           string //
	TargetTaxAmountMinor           string //
	SubtotalMinor                  string //
	NetAmountMinor                 string //
	TaxAmountMinor                 string //
	AmountMinor                    string //
	ProviderParameters             string //
	ResourceChannelNo              string //
	ResourcePoolNo                 string //
	ResourceReservationNo          string //
	ResourceSnapshotNo             string //
	IpQuantity                     string //
	MaximumSnapshotAgeSeconds      string //
	PolicyNo                       string //
	PolicyVersion                  string //
	OldIpSetDigest                 string //
	ExpectedServiceVersion         string //
	CustomerSubjectVersion         string //
	PayerCustomerSubjectVersion    string //
	IdempotencyDigest              string //
	RequestDigest                  string //
	PricingRuleDigest              string //
	ContentDigest                  string //
	EffectiveAt                    string //
	IssuedAt                       string //
	ExpiresAt                      string //
	CreatedAt                      string //
	SourceCode                     string // Actual admission owner: channel retains real attribution; native has no channel identity
	NativeTargetSnapshot           string //
	PreparedPrice                  string // Immutable core price, configuration, merchant policy and local IP reservation facts for native admission
	NativeTargetQuoteNo            string //
}

// channelServiceQuoteColumns holds the columns for the table plugin_idc_core_channel_service_quote.
var channelServiceQuoteColumns = ChannelServiceQuoteColumns{
	Id:                             "id",
	TenantId:                       "tenant_id",
	QuoteNo:                        "quote_no",
	ChannelId:                      "channel_id",
	PayerCustomerNo:                "payer_customer_no",
	ServiceOwnerCustomerNo:         "service_owner_customer_no",
	ServiceNo:                      "service_no",
	ActionCode:                     "action_code",
	CurrencyCode:                   "currency_code",
	DirectionCode:                  "direction_code",
	CurrentConfigurationSnapshotNo: "current_configuration_snapshot_no",
	TargetConfigurationSnapshotNo:  "target_configuration_snapshot_no",
	TargetVersionNo:                "target_version_no",
	TargetBillingRuleNo:            "target_billing_rule_no",
	CurrentRecurringAmountMinor:    "current_recurring_amount_minor",
	TargetRecurringAmountMinor:     "target_recurring_amount_minor",
	TargetSubtotalMinor:            "target_subtotal_minor",
	TargetNetAmountMinor:           "target_net_amount_minor",
	TargetTaxAmountMinor:           "target_tax_amount_minor",
	SubtotalMinor:                  "subtotal_minor",
	NetAmountMinor:                 "net_amount_minor",
	TaxAmountMinor:                 "tax_amount_minor",
	AmountMinor:                    "amount_minor",
	ProviderParameters:             "provider_parameters",
	ResourceChannelNo:              "resource_channel_no",
	ResourcePoolNo:                 "resource_pool_no",
	ResourceReservationNo:          "resource_reservation_no",
	ResourceSnapshotNo:             "resource_snapshot_no",
	IpQuantity:                     "ip_quantity",
	MaximumSnapshotAgeSeconds:      "maximum_snapshot_age_seconds",
	PolicyNo:                       "policy_no",
	PolicyVersion:                  "policy_version",
	OldIpSetDigest:                 "old_ip_set_digest",
	ExpectedServiceVersion:         "expected_service_version",
	CustomerSubjectVersion:         "customer_subject_version",
	PayerCustomerSubjectVersion:    "payer_customer_subject_version",
	IdempotencyDigest:              "idempotency_digest",
	RequestDigest:                  "request_digest",
	PricingRuleDigest:              "pricing_rule_digest",
	ContentDigest:                  "content_digest",
	EffectiveAt:                    "effective_at",
	IssuedAt:                       "issued_at",
	ExpiresAt:                      "expires_at",
	CreatedAt:                      "created_at",
	SourceCode:                     "source_code",
	NativeTargetSnapshot:           "native_target_snapshot",
	PreparedPrice:                  "prepared_price",
	NativeTargetQuoteNo:            "native_target_quote_no",
}

// NewChannelServiceQuoteDao creates and returns a new DAO object for table data access.
func NewChannelServiceQuoteDao(handlers ...gdb.ModelHandler) *ChannelServiceQuoteDao {
	return &ChannelServiceQuoteDao{
		group:    "default",
		table:    "plugin_idc_core_channel_service_quote",
		columns:  channelServiceQuoteColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChannelServiceQuoteDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChannelServiceQuoteDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChannelServiceQuoteDao) Columns() ChannelServiceQuoteColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChannelServiceQuoteDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChannelServiceQuoteDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChannelServiceQuoteDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
