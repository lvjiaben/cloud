// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EntitlementUpgrade is the golang structure for table entitlement_upgrade.
type EntitlementUpgrade struct {
	Id                    int64      `json:"id"                    orm:"id"                       description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"                description:""`
	UpgradeNo             string     `json:"upgradeNo"             orm:"upgrade_no"               description:""`
	CustomerNo            string     `json:"customerNo"            orm:"customer_no"              description:""`
	EntitlementNo         string     `json:"entitlementNo"         orm:"entitlement_no"           description:""`
	ProductNo             string     `json:"productNo"             orm:"product_no"               description:""`
	SourceVersion         int64      `json:"sourceVersion"         orm:"source_version"           description:""`
	SourceConfigurationNo string     `json:"sourceConfigurationNo" orm:"source_configuration_no"  description:""`
	TargetConfigurationNo string     `json:"targetConfigurationNo" orm:"target_configuration_no"  description:""`
	TargetPriceSnapshotNo string     `json:"targetPriceSnapshotNo" orm:"target_price_snapshot_no" description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"            description:""`
	SourcePeriodNetMinor  int64      `json:"sourcePeriodNetMinor"  orm:"source_period_net_minor"  description:""`
	TargetPeriodNetMinor  int64      `json:"targetPeriodNetMinor"  orm:"target_period_net_minor"  description:""`
	ChargeNetMinor        int64      `json:"chargeNetMinor"        orm:"charge_net_minor"         description:""`
	ChargeTaxMinor        int64      `json:"chargeTaxMinor"        orm:"charge_tax_minor"         description:""`
	ChargeTotalMinor      int64      `json:"chargeTotalMinor"      orm:"charge_total_minor"       description:""`
	PeriodStartedAt       *time.Time `json:"periodStartedAt"       orm:"period_started_at"        description:""`
	PeriodExpiresAt       *time.Time `json:"periodExpiresAt"       orm:"period_expires_at"        description:""`
	QuoteExpiresAt        *time.Time `json:"quoteExpiresAt"        orm:"quote_expires_at"         description:""`
	FrozenTarget          string     `json:"frozenTarget"          orm:"frozen_target"            description:""`
	FrozenPrice           string     `json:"frozenPrice"           orm:"frozen_price"             description:""`
	ProviderParameters    string     `json:"providerParameters"    orm:"provider_parameters"      description:""`
	ChannelNo             string     `json:"channelNo"             orm:"channel_no"               description:""`
	PoolNo                string     `json:"poolNo"                orm:"pool_no"                  description:""`
	InvoiceNo             string     `json:"invoiceNo"             orm:"invoice_no"               description:""`
	ProviderTaskNo        string     `json:"providerTaskNo"        orm:"provider_task_no"         description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"              description:""`
	IdempotencyDigest     string     `json:"idempotencyDigest"     orm:"idempotency_digest"       description:""`
	RequestDigest         string     `json:"requestDigest"         orm:"request_digest"           description:""`
	ContentDigest         string     `json:"contentDigest"         orm:"content_digest"           description:""`
	LastErrorCode         string     `json:"lastErrorCode"         orm:"last_error_code"          description:""`
	AttemptCount          int        `json:"attemptCount"          orm:"attempt_count"            description:""`
	AvailableAt           *time.Time `json:"availableAt"           orm:"available_at"             description:""`
	PaidAt                *time.Time `json:"paidAt"                orm:"paid_at"                  description:""`
	CompletedAt           *time.Time `json:"completedAt"           orm:"completed_at"             description:""`
	Version               int64      `json:"version"               orm:"version"                  description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"               description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"               description:""`
}
