// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerAccessoryEntitlement is the golang structure for table server_accessory_entitlement.
type ServerAccessoryEntitlement struct {
	Id                      int64      `json:"id"                      orm:"id"                         description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                  description:""`
	EntitlementNo           string     `json:"entitlementNo"           orm:"entitlement_no"             description:""`
	CustomerNo              string     `json:"customerNo"              orm:"customer_no"                description:""`
	ServiceNo               string     `json:"serviceNo"               orm:"service_no"                 description:""`
	CreateOperationNo       string     `json:"createOperationNo"       orm:"create_operation_no"        description:""`
	ObjectNo                string     `json:"objectNo"                orm:"object_no"                  description:"Resource-owner opaque object number; Provider identifiers never cross this boundary"`
	AccessoryKind           string     `json:"accessoryKind"           orm:"accessory_kind"             description:""`
	RequestedName           string     `json:"requestedName"           orm:"requested_name"             description:""`
	BillingModeCode         string     `json:"billingModeCode"         orm:"billing_mode_code"          description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"                description:""`
	AutoRenewEnabled        bool       `json:"autoRenewEnabled"        orm:"auto_renew_enabled"         description:""`
	CurrencyCode            string     `json:"currencyCode"            orm:"currency_code"              description:""`
	PeriodUnitCode          string     `json:"periodUnitCode"          orm:"period_unit_code"           description:""`
	PeriodCount             int        `json:"periodCount"             orm:"period_count"               description:""`
	CurrentPeriodStartAt    *time.Time `json:"currentPeriodStartAt"    orm:"current_period_start_at"    description:""`
	CurrentPeriodEndAt      *time.Time `json:"currentPeriodEndAt"      orm:"current_period_end_at"      description:""`
	PendingInvoiceNo        string     `json:"pendingInvoiceNo"        orm:"pending_invoice_no"         description:""`
	PendingPolicyNo         string     `json:"pendingPolicyNo"         orm:"pending_policy_no"          description:""`
	PendingPolicyVersion    int64      `json:"pendingPolicyVersion"    orm:"pending_policy_version"     description:""`
	PendingPeriodUnitCode   string     `json:"pendingPeriodUnitCode"   orm:"pending_period_unit_code"   description:""`
	PendingPeriodCount      int        `json:"pendingPeriodCount"      orm:"pending_period_count"       description:""`
	PendingTotalAmountMinor int64      `json:"pendingTotalAmountMinor" orm:"pending_total_amount_minor" description:""`
	LastRenewalAttemptAt    *time.Time `json:"lastRenewalAttemptAt"    orm:"last_renewal_attempt_at"    description:""`
	BaselineObjectNos       string     `json:"baselineObjectNos"       orm:"baseline_object_nos"        description:""`
	BaselineSnapshotNo      string     `json:"baselineSnapshotNo"      orm:"baseline_snapshot_no"       description:""`
	BaselineObservedAt      *time.Time `json:"baselineObservedAt"      orm:"baseline_observed_at"       description:""`
	BindingAttemptCount     int        `json:"bindingAttemptCount"     orm:"binding_attempt_count"      description:""`
	LastBindingAttemptAt    *time.Time `json:"lastBindingAttemptAt"    orm:"last_binding_attempt_at"    description:""`
	DeletedAt               *time.Time `json:"deletedAt"               orm:"deleted_at"                 description:""`
	Version                 int64      `json:"version"                 orm:"version"                    description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                 description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                 description:""`
	SubjectTypeCode         string     `json:"subjectTypeCode"         orm:"subject_type_code"          description:""`
	SubjectNo               string     `json:"subjectNo"               orm:"subject_no"                 description:""`
}
