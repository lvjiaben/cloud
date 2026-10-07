// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaskManualTakeover is the golang structure of table plugin_idc_resource_task_manual_takeover for DAO operations like Where/Data.
type TaskManualTakeover struct {
	g.Meta         `orm:"table:plugin_idc_resource_task_manual_takeover, do:true"`
	Id             any        //
	TenantId       any        //
	TakeoverNo     any        //
	TaskNo         any        //
	FromStatusCode any        //
	TaskVersion    any        //
	ReasonCode     any        //
	RequestDigest  any        // Canonical digest of actor, task version, reason, and sorted evidence numbers for exact safe replay
	EvidenceDigest any        // Canonical digest of immutable task evidence numbers and their redacted contents
	EvidenceCount  any        //
	AssignedUserId any        //
	CreatedAt      *time.Time //
}
