// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaskEvidence is the golang structure of table plugin_idc_resource_task_evidence for DAO operations like Where/Data.
type TaskEvidence struct {
	g.Meta       `orm:"table:plugin_idc_resource_task_evidence, do:true"`
	Id           any        //
	TenantId     any        //
	EvidenceNo   any        //
	TaskNo       any        //
	SourceKind   any        //
	EvidenceKind any        //
	Reference    any        // Bounded upstream task or resource reference; never a secret handle or raw payload
	Summary      any        // Bounded credential-scanned Provider or owner summary; never a raw response
	ObservedAt   *time.Time //
	CreatedAt    *time.Time //
}
