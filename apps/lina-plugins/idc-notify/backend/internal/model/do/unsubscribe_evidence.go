// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// UnsubscribeEvidence is the golang structure of table plugin_idc_notify_unsubscribe_evidence for DAO operations like Where/Data.
type UnsubscribeEvidence struct {
	g.Meta         `orm:"table:plugin_idc_notify_unsubscribe_evidence, do:true"`
	TenantId       any        //
	EvidenceNo     any        //
	OwnerUserId    any        //
	CustomerNo     any        //
	ContactNo      any        //
	ChannelCode    any        //
	TopicCode      any        //
	SourceCode     any        //
	EvidenceDigest any        //
	OccurredAt     *time.Time //
	CreatedAt      *time.Time //
}
