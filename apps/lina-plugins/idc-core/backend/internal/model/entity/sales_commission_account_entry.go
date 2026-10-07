// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionAccountEntry is the golang structure for table sales_commission_account_entry.
type SalesCommissionAccountEntry struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	EntryNo             string     `json:"entryNo"             orm:"entry_no"              description:""`
	AccountNo           string     `json:"accountNo"           orm:"account_no"            description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	EntrySequence       int64      `json:"entrySequence"       orm:"entry_sequence"        description:""`
	EntryTypeCode       string     `json:"entryTypeCode"       orm:"entry_type_code"       description:""`
	SourceTypeCode      string     `json:"sourceTypeCode"      orm:"source_type_code"      description:""`
	SourceReference     string     `json:"sourceReference"     orm:"source_reference"      description:""`
	SourceDigest        string     `json:"sourceDigest"        orm:"source_digest"         description:""`
	DeltaCreditedMinor  int64      `json:"deltaCreditedMinor"  orm:"delta_credited_minor"  description:""`
	DeltaPendingMinor   int64      `json:"deltaPendingMinor"   orm:"delta_pending_minor"   description:""`
	DeltaAvailableMinor int64      `json:"deltaAvailableMinor" orm:"delta_available_minor" description:""`
	DeltaFrozenMinor    int64      `json:"deltaFrozenMinor"    orm:"delta_frozen_minor"    description:""`
	DeltaPaidMinor      int64      `json:"deltaPaidMinor"      orm:"delta_paid_minor"      description:""`
	DeltaReversedMinor  int64      `json:"deltaReversedMinor"  orm:"delta_reversed_minor"  description:""`
	AfterCreditedMinor  int64      `json:"afterCreditedMinor"  orm:"after_credited_minor"  description:""`
	AfterPendingMinor   int64      `json:"afterPendingMinor"   orm:"after_pending_minor"   description:""`
	AfterAvailableMinor int64      `json:"afterAvailableMinor" orm:"after_available_minor" description:""`
	AfterFrozenMinor    int64      `json:"afterFrozenMinor"    orm:"after_frozen_minor"    description:""`
	AfterPaidMinor      int64      `json:"afterPaidMinor"      orm:"after_paid_minor"      description:""`
	AfterReversedMinor  int64      `json:"afterReversedMinor"  orm:"after_reversed_minor"  description:""`
	OccurredAt          *time.Time `json:"occurredAt"          orm:"occurred_at"           description:""`
}
