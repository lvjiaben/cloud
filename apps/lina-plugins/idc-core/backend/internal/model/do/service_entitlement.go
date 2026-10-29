// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// ServiceEntitlement is the golang structure of table plugin_idc_core_service_entitlement for DAO operations like Where/Data.
type ServiceEntitlement struct {
	g.Meta                      `orm:"table:plugin_idc_core_service_entitlement, do:true"`
	TenantId                    any //
	Id                          any //
	EntitlementNo               any //
	DeliveryNo                  any //
	SourceIntentNo              any //
	CustomerNo                  any //
	OrderNo                     any //
	OrderItemNo                 any //
	ProductNo                   any //
	VersionNo                   any //
	UnitSequence                any //
	ConfigurationSnapshotNo     any //
	InterfaceInstanceNo         any //
	PluginId                    any //
	PluginReleaseVersion        any //
	ContractVersion             any //
	DeclarationDigest           any //
	StatusCode                  any //
	StartedAt                   any //
	ExpiresAt                   any //
	Version                     any //
	CreatedAt                   any //
	UpdatedAt                   any //
	RenewalModeCode             any //
	RenewalMethod               any //
	EntitlementOwnerId          any //
	CurrencyCode                any //
	BillingRuleNo               any //
	ChargeTypeCode              any //
	PeriodUnitCode              any //
	PeriodCount                 any //
	CurrentRenewalPolicyNo      any //
	CurrentRenewalPolicyVersion any //
	CurrentPriceSnapshotNo      any //
	LastRenewalOperationNo      any //
	SupportAssociationReference any //
	UpstreamOrderReference      any //
	UpstreamDeliveryNo          any //
	UpstreamServiceNo           any //
	CustomerRemark              any // Current customer non-sensitive service note
	CustomerRemarkRevision      any // Independent optimistic revision for the customer service note
}
