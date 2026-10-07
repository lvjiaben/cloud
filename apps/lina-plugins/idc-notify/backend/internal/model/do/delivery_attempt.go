// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// DeliveryAttempt is the golang structure of table plugin_idc_notify_delivery_attempt for DAO operations like Where/Data.
type DeliveryAttempt struct {
	g.Meta                  `orm:"table:plugin_idc_notify_delivery_attempt, do:true"`
	TenantId                any //
	AttemptNo               any //
	TaskNo                  any //
	RetrySequence           any //
	AttemptFence            any //
	CrossedExternalBoundary any //
	ResultClass             any //
	EvidenceDigest          any //
	ProviderReference       any //
	StartedAt               any //
	FinishedAt              any //
}
