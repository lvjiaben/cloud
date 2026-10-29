// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServerAccessoryEntitlement is the golang structure of table plugin_idc_core_server_accessory_entitlement for DAO operations like Where/Data.
type ServerAccessoryEntitlement struct {
	g.Meta                  `orm:"table:plugin_idc_core_server_accessory_entitlement, do:true"`
	Id                      any        //
	TenantId                any        //
	EntitlementNo           any        //
	CustomerNo              any        //
	ServiceNo               any        //
	CreateOperationNo       any        //
	ObjectNo                any        // Resource-owner opaque object number; Provider identifiers never cross this boundary
	AccessoryKind           any        //
	RequestedName           any        //
	BillingModeCode         any        //
	StatusCode              any        //
	AutoRenewEnabled        any        //
	CurrencyCode            any        //
	PeriodUnitCode          any        //
	PeriodCount             any        //
	CurrentPeriodStartAt    *time.Time //
	CurrentPeriodEndAt      *time.Time //
	PendingInvoiceNo        any        //
	PendingPolicyNo         any        //
	PendingPolicyVersion    any        //
	PendingPeriodUnitCode   any        //
	PendingPeriodCount      any        //
	PendingTotalAmountMinor any        //
	LastRenewalAttemptAt    *time.Time //
	BaselineObjectNos       any        //
	BaselineSnapshotNo      any        //
	BaselineObservedAt      *time.Time //
	BindingAttemptCount     any        //
	LastBindingAttemptAt    *time.Time //
	DeletedAt               *time.Time //
	Version                 any        //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
	SubjectTypeCode         any        //
	SubjectNo               any        //
}
