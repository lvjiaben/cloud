// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContentScanAttempt is the golang structure of table plugin_idc_core_content_scan_attempt for DAO operations like Where/Data.
type ContentScanAttempt struct {
	g.Meta              `orm:"table:plugin_idc_core_content_scan_attempt, do:true"`
	Id                  any        //
	TenantId            any        //
	AttemptNo           any        //
	FileNo              any        //
	CandidateGeneration any        //
	StatusCode          any        //
	QueuedAt            *time.Time //
	StartedAt           *time.Time //
	LeaseExpiresAt      *time.Time //
	FinishedAt          *time.Time //
	RiskCode            any        //
	CreatedBy           any        //
	Version             any        //
}
