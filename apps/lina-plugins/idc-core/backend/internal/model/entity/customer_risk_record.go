// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerRiskRecord is the golang structure for table customer_risk_record.
type CustomerRiskRecord struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	RiskNo            string     `json:"riskNo"            orm:"risk_no"            description:""`
	CustomerNo        string     `json:"customerNo"        orm:"customer_no"        description:""`
	CategoryCode      string     `json:"categoryCode"      orm:"category_code"      description:""`
	SourceCode        string     `json:"sourceCode"        orm:"source_code"        description:""`
	LevelCode         string     `json:"levelCode"         orm:"level_code"         description:""`
	ScoreBasisPoints  *int       `json:"scoreBasisPoints"  orm:"score_basis_points" description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"        description:""`
	Summary           string     `json:"summary"           orm:"summary"            description:""`
	EvidenceReference string     `json:"evidenceReference" orm:"evidence_reference" description:""`
	OccurredAt        *time.Time `json:"occurredAt"        orm:"occurred_at"        description:""`
	AcknowledgedBy    int        `json:"acknowledgedBy"    orm:"acknowledged_by"    description:""`
	AcknowledgedAt    *time.Time `json:"acknowledgedAt"    orm:"acknowledged_at"    description:""`
	ResolvedBy        int        `json:"resolvedBy"        orm:"resolved_by"        description:""`
	ResolvedAt        *time.Time `json:"resolvedAt"        orm:"resolved_at"        description:""`
	Resolution        string     `json:"resolution"        orm:"resolution"         description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
