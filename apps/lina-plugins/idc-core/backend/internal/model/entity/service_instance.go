// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceInstance is the golang structure for table service_instance.
type ServiceInstance struct {
	Id                            int64      `json:"id"                            orm:"id"                               description:""`
	TenantId                      int        `json:"tenantId"                      orm:"tenant_id"                        description:""`
	ServiceNo                     string     `json:"serviceNo"                     orm:"service_no"                       description:""`
	CustomerNo                    string     `json:"customerNo"                    orm:"customer_no"                      description:""`
	SourceCode                    string     `json:"sourceCode"                    orm:"source_code"                      description:""`
	SourceFulfillmentNo           *string    `json:"sourceFulfillmentNo"           orm:"source_fulfillment_no"            description:""`
	OrderNo                       *string    `json:"orderNo"                       orm:"order_no"                         description:""`
	OrderItemNo                   *string    `json:"orderItemNo"                   orm:"order_item_no"                    description:""`
	ProductNo                     string     `json:"productNo"                     orm:"product_no"                       description:""`
	VersionNo                     string     `json:"versionNo"                     orm:"version_no"                       description:""`
	BillingRuleNo                 string     `json:"billingRuleNo"                 orm:"billing_rule_no"                  description:""`
	ConfigurationSnapshotNo       string     `json:"configurationSnapshotNo"       orm:"configuration_snapshot_no"        description:""`
	ProductTypeCode               string     `json:"productTypeCode"               orm:"product_type_code"                description:""`
	CurrencyCode                  string     `json:"currencyCode"                  orm:"currency_code"                    description:""`
	ResourceOwnerCode             string     `json:"resourceOwnerCode"             orm:"resource_owner_code"              description:""`
	BindingNo                     string     `json:"bindingNo"                     orm:"binding_no"                       description:""`
	PoolNo                        string     `json:"poolNo"                        orm:"pool_no"                          description:""`
	ChannelNo                     *string    `json:"channelNo"                     orm:"channel_no"                       description:""`
	EntitlementStatusCode         string     `json:"entitlementStatusCode"         orm:"entitlement_status_code"          description:""`
	RuntimeStatusCode             string     `json:"runtimeStatusCode"             orm:"runtime_status_code"              description:""`
	ProviderTaskStatusCode        string     `json:"providerTaskStatusCode"        orm:"provider_task_status_code"        description:""`
	RenewalModeCode               string     `json:"renewalModeCode"               orm:"renewal_mode_code"                description:""`
	RenewalCapabilityEvidenceNo   *string    `json:"renewalCapabilityEvidenceNo"   orm:"renewal_capability_evidence_no"   description:""`
	RenewalCapabilityDigest       *string    `json:"renewalCapabilityDigest"       orm:"renewal_capability_digest"        description:""`
	RenewalCapabilityObservedAt   *time.Time `json:"renewalCapabilityObservedAt"   orm:"renewal_capability_observed_at"   description:""`
	RenewalCapabilityExpiresAt    *time.Time `json:"renewalCapabilityExpiresAt"    orm:"renewal_capability_expires_at"    description:""`
	PeriodUnitCode                string     `json:"periodUnitCode"                orm:"period_unit_code"                 description:""`
	PeriodCount                   int        `json:"periodCount"                   orm:"period_count"                     description:""`
	EntitlementStartedAt          *time.Time `json:"entitlementStartedAt"          orm:"entitlement_started_at"           description:""`
	EntitlementExpiresAt          *time.Time `json:"entitlementExpiresAt"          orm:"entitlement_expires_at"           description:""`
	CurrentPriceSnapshotNo        *string    `json:"currentPriceSnapshotNo"        orm:"current_price_snapshot_no"        description:""`
	CurrentRenewalPolicyNo        *string    `json:"currentRenewalPolicyNo"        orm:"current_renewal_policy_no"        description:""`
	CurrentRenewalPolicyVersion   *int64     `json:"currentRenewalPolicyVersion"   orm:"current_renewal_policy_version"   description:""`
	LastBusinessOperationNo       *string    `json:"lastBusinessOperationNo"       orm:"last_business_operation_no"       description:""`
	Version                       int64      `json:"version"                       orm:"version"                          description:""`
	CreatedAt                     *time.Time `json:"createdAt"                     orm:"created_at"                       description:""`
	UpdatedAt                     *time.Time `json:"updatedAt"                     orm:"updated_at"                       description:""`
	CurrentLifecyclePolicyNo      *string    `json:"currentLifecyclePolicyNo"      orm:"current_lifecycle_policy_no"      description:""`
	CurrentLifecyclePolicyVersion *int64     `json:"currentLifecyclePolicyVersion" orm:"current_lifecycle_policy_version" description:""`
	CustomerAlias                 string     `json:"customerAlias"                 orm:"customer_alias"                   description:""`
	CustomerRemark                string     `json:"customerRemark"                orm:"customer_remark"                  description:""`
	OwnershipStartedAt            *time.Time `json:"ownershipStartedAt"            orm:"ownership_started_at"             description:""`
}
