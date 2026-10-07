// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerBlacklistEntry is the golang structure of table plugin_idc_core_customer_blacklist_entry for DAO operations like Where/Data.
type CustomerBlacklistEntry struct {
	g.Meta            `orm:"table:plugin_idc_core_customer_blacklist_entry, do:true"`
	Id                any        //
	TenantId          any        //
	EntryNo           any        //
	CustomerNo        any        //
	ScopeCode         any        //
	ReasonCode        any        //
	ReasonDetail      any        //
	EvidenceReference any        //
	RiskLevelCode     any        //
	StatusCode        any        //
	StartsAt          *time.Time //
	ExpiresAt         *time.Time //
	CreatedBy         any        //
	ApprovedBy        any        //
	ApprovedAt        *time.Time //
	RevokedBy         any        //
	RevokedAt         *time.Time //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
