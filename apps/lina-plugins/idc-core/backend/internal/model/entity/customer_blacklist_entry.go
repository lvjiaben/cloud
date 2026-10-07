// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerBlacklistEntry is the golang structure for table customer_blacklist_entry.
type CustomerBlacklistEntry struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	EntryNo           string     `json:"entryNo"           orm:"entry_no"           description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"        description:""`
	ScopeCode         string     `json:"scopeCode"         orm:"scope_code"         description:""`
	ReasonCode        string     `json:"reasonCode"        orm:"reason_code"        description:""`
	ReasonDetail      string     `json:"reasonDetail"      orm:"reason_detail"      description:""`
	EvidenceReference string     `json:"evidenceReference" orm:"evidence_reference" description:""`
	RiskLevelCode     string     `json:"riskLevelCode"     orm:"risk_level_code"    description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"        description:""`
	StartsAt          *time.Time `json:"startsAt"          orm:"starts_at"          description:""`
	ExpiresAt         *time.Time `json:"expiresAt"         orm:"expires_at"         description:""`
	CreatedBy         int        `json:"createdBy"         orm:"created_by"         description:""`
	ApprovedBy        int        `json:"approvedBy"        orm:"approved_by"        description:""`
	ApprovedAt        *time.Time `json:"approvedAt"        orm:"approved_at"        description:""`
	RevokedBy         int        `json:"revokedBy"         orm:"revoked_by"         description:""`
	RevokedAt         *time.Time `json:"revokedAt"         orm:"revoked_at"         description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
