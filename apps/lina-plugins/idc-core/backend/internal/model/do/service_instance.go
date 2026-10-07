// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceInstance is the golang structure of table plugin_idc_core_service_instance for DAO operations like Where/Data.
type ServiceInstance struct {
	g.Meta                        `orm:"table:plugin_idc_core_service_instance, do:true"`
	Id                            any        //
	TenantId                      any        //
	ServiceNo                     any        //
	CustomerNo                    any        //
	SourceCode                    any        //
	SourceFulfillmentNo           *string    //
	OrderNo                       *string    //
	OrderItemNo                   *string    //
	ProductNo                     any        //
	VersionNo                     any        //
	BillingRuleNo                 any        //
	ConfigurationSnapshotNo       any        //
	ProductTypeCode               any        //
	CurrencyCode                  any        //
	ResourceOwnerCode             any        //
	BindingNo                     any        //
	PoolNo                        any        //
	ChannelNo                     *string    //
	EntitlementStatusCode         any        //
	RuntimeStatusCode             any        //
	ProviderTaskStatusCode        any        //
	RenewalModeCode               any        //
	RenewalCapabilityEvidenceNo   *string    //
	RenewalCapabilityDigest       *string    //
	RenewalCapabilityObservedAt   *time.Time //
	RenewalCapabilityExpiresAt    *time.Time //
	PeriodUnitCode                any        //
	PeriodCount                   any        //
	EntitlementStartedAt          *time.Time //
	EntitlementExpiresAt          *time.Time //
	CurrentPriceSnapshotNo        *string    //
	CurrentRenewalPolicyNo        *string    //
	CurrentRenewalPolicyVersion   *int64     //
	LastBusinessOperationNo       *string    //
	Version                       any        //
	CreatedAt                     *time.Time //
	UpdatedAt                     *time.Time //
	CurrentLifecyclePolicyNo      *string    //
	CurrentLifecyclePolicyVersion *int64     //
	CustomerAlias                 any        //
	CustomerRemark                any        //
	OwnershipStartedAt            *time.Time //
}
