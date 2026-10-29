// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FulfillmentNonAcceptanceVerification is the golang structure of table plugin_idc_core_fulfillment_non_acceptance_verification for DAO operations like Where/Data.
type FulfillmentNonAcceptanceVerification struct {
	g.Meta              `orm:"table:plugin_idc_core_fulfillment_non_acceptance_verification, do:true"`
	TenantId            any        //
	ProviderTaskNo      any        //
	IntentNo            any        //
	TaskNo              any        //
	PreviousTaskVersion any        //
	ResultingVersion    any        //
	IdempotencyKey      any        //
	RequestDigest       any        //
	ActorUserId         any        //
	SafeNote            any        //
	VerifiedAt          *time.Time //
}
