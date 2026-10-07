// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerRiskRecord is the golang structure of table plugin_idc_core_customer_risk_record for DAO operations like Where/Data.
type CustomerRiskRecord struct {
	g.Meta            `orm:"table:plugin_idc_core_customer_risk_record, do:true"`
	Id                any        //
	TenantId          any        //
	RiskNo            any        //
	CustomerNo        any        //
	CategoryCode      any        //
	SourceCode        any        //
	LevelCode         any        //
	ScoreBasisPoints  *int       //
	StatusCode        any        //
	Summary           any        //
	EvidenceReference any        //
	OccurredAt        *time.Time //
	AcknowledgedBy    any        //
	AcknowledgedAt    *time.Time //
	ResolvedBy        any        //
	ResolvedAt        *time.Time //
	Resolution        any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
