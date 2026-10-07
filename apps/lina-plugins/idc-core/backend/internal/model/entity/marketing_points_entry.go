// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingPointsEntry is the golang structure for table marketing_points_entry.
type MarketingPointsEntry struct {
	Id                int64      `json:"id"                orm:"id"                   description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"            description:""`
	EntryNo           string     `json:"entryNo"           orm:"entry_no"             description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"          description:""`
	LotNo             *string    `json:"lotNo"             orm:"lot_no"               description:""`
	EntryTypeCode     string     `json:"entryTypeCode"     orm:"entry_type_code"      description:""`
	AvailableDelta    int64      `json:"availableDelta"    orm:"available_delta"      description:""`
	FrozenDelta       int64      `json:"frozenDelta"       orm:"frozen_delta"         description:""`
	DebtDelta         int64      `json:"debtDelta"         orm:"debt_delta"           description:""`
	SourceTypeCode    string     `json:"sourceTypeCode"    orm:"source_type_code"     description:""`
	SourceReference   string     `json:"sourceReference"   orm:"source_reference"     description:""`
	EntryLineNo       int        `json:"entryLineNo"       orm:"entry_line_no"        description:""`
	ReversalOfEntryNo *string    `json:"reversalOfEntryNo" orm:"reversal_of_entry_no" description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"           description:""`
}
