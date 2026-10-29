// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// InboxDeliveryEvidence is the golang structure of table plugin_shared_services_inbox_delivery_evidence for DAO operations like Where/Data.
type InboxDeliveryEvidence struct {
	g.Meta         `orm:"table:plugin_shared_services_inbox_delivery_evidence, do:true"`
	Id             any        //
	TenantId       any        //
	EvidenceNo     any        //
	MessageNo      any        //
	CallerPluginId any        //
	EvidenceType   any        //
	ContentDigest  any        //
	CreatedAt      *time.Time //
	HostMessageId  any        //
}
