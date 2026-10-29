// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerAccessoryQuote is the golang structure for table server_accessory_quote.
type ServerAccessoryQuote struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	QuoteNo                string     `json:"quoteNo"                orm:"quote_no"                 description:""`
	CustomerNo             string     `json:"customerNo"             orm:"customer_no"              description:""`
	ServiceNo              string     `json:"serviceNo"              orm:"service_no"               description:""`
	ActionCode             string     `json:"actionCode"             orm:"action_code"              description:""`
	AccessoryKind          string     `json:"accessoryKind"          orm:"accessory_kind"           description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	RequestedQuantity      int        `json:"requestedQuantity"      orm:"requested_quantity"       description:""`
	ExistingObjectCount    int        `json:"existingObjectCount"    orm:"existing_object_count"    description:""`
	ReservedCreateCount    int        `json:"reservedCreateCount"    orm:"reserved_create_count"    description:""`
	FreeCountPerServer     int        `json:"freeCountPerServer"     orm:"free_count_per_server"    description:""`
	ChargedQuantity        int        `json:"chargedQuantity"        orm:"charged_quantity"         description:""`
	UnitAmountMinor        int64      `json:"unitAmountMinor"        orm:"unit_amount_minor"        description:""`
	TotalAmountMinor       int64      `json:"totalAmountMinor"       orm:"total_amount_minor"       description:""`
	PeriodUnitCode         string     `json:"periodUnitCode"         orm:"period_unit_code"         description:""`
	PeriodCount            int        `json:"periodCount"            orm:"period_count"             description:""`
	PolicyNo               string     `json:"policyNo"               orm:"policy_no"                description:""`
	PolicyVersion          int64      `json:"policyVersion"          orm:"policy_version"           description:""`
	IdempotencyDigest      string     `json:"idempotencyDigest"      orm:"idempotency_digest"       description:""`
	RequestDigest          string     `json:"requestDigest"          orm:"request_digest"           description:""`
	ContentDigest          string     `json:"contentDigest"          orm:"content_digest"           description:""`
	IssuedAt               *time.Time `json:"issuedAt"               orm:"issued_at"                description:""`
	ExpiresAt              *time.Time `json:"expiresAt"              orm:"expires_at"               description:""`
	Parameters             string     `json:"parameters"             orm:"parameters"               description:"Canonical provider parameters frozen by the customer-visible quote"`
	CustomerSubjectVersion int64      `json:"customerSubjectVersion" orm:"customer_subject_version" description:""`
	ExpectedServiceVersion int64      `json:"expectedServiceVersion" orm:"expected_service_version" description:""`
	SubtotalMinor          int64      `json:"subtotalMinor"          orm:"subtotal_minor"           description:""`
	NetAmountMinor         int64      `json:"netAmountMinor"         orm:"net_amount_minor"         description:""`
	TaxAmountMinor         int64      `json:"taxAmountMinor"         orm:"tax_amount_minor"         description:""`
	TaxPolicyNo            string     `json:"taxPolicyNo"            orm:"tax_policy_no"            description:""`
	TaxTreatmentCode       string     `json:"taxTreatmentCode"       orm:"tax_treatment_code"       description:""`
	TaxRateBasisPoints     int        `json:"taxRateBasisPoints"     orm:"tax_rate_basis_points"    description:""`
	TaxCategoryCode        string     `json:"taxCategoryCode"        orm:"tax_category_code"        description:""`
	RoundingScopeCode      string     `json:"roundingScopeCode"      orm:"rounding_scope_code"      description:""`
	RoundingModeCode       string     `json:"roundingModeCode"       orm:"rounding_mode_code"       description:""`
	RoundingQuantumMinor   int64      `json:"roundingQuantumMinor"   orm:"rounding_quantum_minor"   description:""`
	BaselineObjectNos      string     `json:"baselineObjectNos"      orm:"baseline_object_nos"      description:""`
	BaselineSnapshotNo     string     `json:"baselineSnapshotNo"     orm:"baseline_snapshot_no"     description:""`
	BaselineObservedAt     *time.Time `json:"baselineObservedAt"     orm:"baseline_observed_at"     description:""`
	SubjectTypeCode        string     `json:"subjectTypeCode"        orm:"subject_type_code"        description:""`
	SubjectNo              string     `json:"subjectNo"              orm:"subject_no"               description:""`
	ExpectedSubjectVersion int64      `json:"expectedSubjectVersion" orm:"expected_subject_version" description:""`
	NativeTargetSnapshot   string     `json:"nativeTargetSnapshot"   orm:"native_target_snapshot"   description:"Source-validated frozen native resource route, directory and local server evidence; never accepted from HTTP"`
}
