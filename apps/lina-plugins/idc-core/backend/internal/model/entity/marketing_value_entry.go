// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingValueEntry is the golang structure for table marketing_value_entry.
type MarketingValueEntry struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	EntryNo             string     `json:"entryNo"             orm:"entry_no"              description:""`
	GrantNo             string     `json:"grantNo"             orm:"grant_no"              description:""`
	CustomerNo          string     `json:"customerNo"          orm:"customer_no"           description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	EntryTypeCode       string     `json:"entryTypeCode"       orm:"entry_type_code"       description:""`
	PendingDeltaMinor   int64      `json:"pendingDeltaMinor"   orm:"pending_delta_minor"   description:""`
	AvailableDeltaMinor int64      `json:"availableDeltaMinor" orm:"available_delta_minor" description:""`
	FrozenDeltaMinor    int64      `json:"frozenDeltaMinor"    orm:"frozen_delta_minor"    description:""`
	ConsumedDeltaMinor  int64      `json:"consumedDeltaMinor"  orm:"consumed_delta_minor"  description:""`
	ExpiredDeltaMinor   int64      `json:"expiredDeltaMinor"   orm:"expired_delta_minor"   description:""`
	ReversedDeltaMinor  int64      `json:"reversedDeltaMinor"  orm:"reversed_delta_minor"  description:""`
	SourceTypeCode      string     `json:"sourceTypeCode"      orm:"source_type_code"      description:""`
	SourceReference     string     `json:"sourceReference"     orm:"source_reference"      description:""`
	ReversalOfEntryNo   *string    `json:"reversalOfEntryNo"   orm:"reversal_of_entry_no"  description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
