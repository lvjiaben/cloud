// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceEntitlement is the golang structure for table service_entitlement.
type ServiceEntitlement struct {
	TenantId                    int        `json:"tenantId"                    orm:"tenant_id"                      description:""`
	Id                          int64      `json:"id"                          orm:"id"                             description:""`
	EntitlementNo               string     `json:"entitlementNo"               orm:"entitlement_no"                 description:""`
	DeliveryNo                  string     `json:"deliveryNo"                  orm:"delivery_no"                    description:""`
	SourceIntentNo              string     `json:"sourceIntentNo"              orm:"source_intent_no"               description:""`
	CustomerNo                  string     `json:"customerNo"                  orm:"customer_no"                    description:""`
	OrderNo                     string     `json:"orderNo"                     orm:"order_no"                       description:""`
	OrderItemNo                 string     `json:"orderItemNo"                 orm:"order_item_no"                  description:""`
	ProductNo                   string     `json:"productNo"                   orm:"product_no"                     description:""`
	VersionNo                   string     `json:"versionNo"                   orm:"version_no"                     description:""`
	UnitSequence                int        `json:"unitSequence"                orm:"unit_sequence"                  description:""`
	ConfigurationSnapshotNo     string     `json:"configurationSnapshotNo"     orm:"configuration_snapshot_no"      description:""`
	InterfaceInstanceNo         string     `json:"interfaceInstanceNo"         orm:"interface_instance_no"          description:""`
	PluginId                    string     `json:"pluginId"                    orm:"plugin_id"                      description:""`
	PluginReleaseVersion        string     `json:"pluginReleaseVersion"        orm:"plugin_release_version"         description:""`
	ContractVersion             string     `json:"contractVersion"             orm:"contract_version"               description:""`
	DeclarationDigest           string     `json:"declarationDigest"           orm:"declaration_digest"             description:""`
	StatusCode                  string     `json:"statusCode"                  orm:"status_code"                    description:""`
	StartedAt                   *time.Time `json:"startedAt"                   orm:"started_at"                     description:""`
	ExpiresAt                   *time.Time `json:"expiresAt"                   orm:"expires_at"                     description:""`
	Version                     int64      `json:"version"                     orm:"version"                        description:""`
	CreatedAt                   *time.Time `json:"createdAt"                   orm:"created_at"                     description:""`
	UpdatedAt                   *time.Time `json:"updatedAt"                   orm:"updated_at"                     description:""`
	RenewalModeCode             string     `json:"renewalModeCode"             orm:"renewal_mode_code"              description:""`
	RenewalMethod               string     `json:"renewalMethod"               orm:"renewal_method"                 description:""`
	EntitlementOwnerId          string     `json:"entitlementOwnerId"          orm:"entitlement_owner_id"           description:""`
	CurrencyCode                string     `json:"currencyCode"                orm:"currency_code"                  description:""`
	BillingRuleNo               string     `json:"billingRuleNo"               orm:"billing_rule_no"                description:""`
	ChargeTypeCode              string     `json:"chargeTypeCode"              orm:"charge_type_code"               description:""`
	PeriodUnitCode              string     `json:"periodUnitCode"              orm:"period_unit_code"               description:""`
	PeriodCount                 int        `json:"periodCount"                 orm:"period_count"                   description:""`
	CurrentRenewalPolicyNo      string     `json:"currentRenewalPolicyNo"      orm:"current_renewal_policy_no"      description:""`
	CurrentRenewalPolicyVersion int64      `json:"currentRenewalPolicyVersion" orm:"current_renewal_policy_version" description:""`
	CurrentPriceSnapshotNo      string     `json:"currentPriceSnapshotNo"      orm:"current_price_snapshot_no"      description:""`
	LastRenewalOperationNo      string     `json:"lastRenewalOperationNo"      orm:"last_renewal_operation_no"      description:""`
	SupportAssociationReference string     `json:"supportAssociationReference" orm:"support_association_reference"  description:""`
	UpstreamOrderReference      string     `json:"upstreamOrderReference"      orm:"upstream_order_reference"       description:""`
	UpstreamDeliveryNo          string     `json:"upstreamDeliveryNo"          orm:"upstream_delivery_no"           description:""`
	UpstreamServiceNo           string     `json:"upstreamServiceNo"           orm:"upstream_service_no"            description:""`
}
