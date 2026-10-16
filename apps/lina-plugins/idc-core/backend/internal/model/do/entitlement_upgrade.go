// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EntitlementUpgrade is the golang structure of table plugin_idc_core_entitlement_upgrade for DAO operations like Where/Data.
type EntitlementUpgrade struct {
	g.Meta                `orm:"table:plugin_idc_core_entitlement_upgrade, do:true"`
	Id                    any        //
	TenantId              any        //
	UpgradeNo             any        //
	CustomerNo            any        //
	EntitlementNo         any        //
	ProductNo             any        //
	SourceVersion         any        //
	SourceConfigurationNo any        //
	TargetConfigurationNo any        //
	TargetPriceSnapshotNo any        //
	CurrencyCode          any        //
	SourcePeriodNetMinor  any        //
	TargetPeriodNetMinor  any        //
	ChargeNetMinor        any        //
	ChargeTaxMinor        any        //
	ChargeTotalMinor      any        //
	PeriodStartedAt       *time.Time //
	PeriodExpiresAt       *time.Time //
	QuoteExpiresAt        *time.Time //
	FrozenTarget          any        //
	FrozenPrice           any        //
	ProviderParameters    any        //
	ChannelNo             any        //
	PoolNo                any        //
	InvoiceNo             any        //
	ProviderTaskNo        any        //
	StatusCode            any        //
	IdempotencyDigest     any        //
	RequestDigest         any        //
	ContentDigest         any        //
	LastErrorCode         any        //
	AttemptCount          any        //
	AvailableAt           *time.Time //
	PaidAt                *time.Time //
	CompletedAt           *time.Time //
	Version               any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
}
