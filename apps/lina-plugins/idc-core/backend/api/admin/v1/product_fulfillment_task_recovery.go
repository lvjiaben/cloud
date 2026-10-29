// This file declares the audited, query-first recovery of resource preparation failures.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

// ProductFulfillmentTaskRecoveryReq cannot replay accepted or unknown supplier writes.
type ProductFulfillmentTaskRecoveryReq struct {
	g.Meta `path:"/admin/product-fulfillment/tasks/{intentNo}/recoveries" method:"post" tags:"IDC Product Fulfillment" summary:"Recover failed provisioning preparation" dc:"Append an independently authorized recovery command for a paid resource-owner task that failed before submission. Preserve cumulative attempts, recheck prerequisites and restart query-first. Unknown, accepted, canceled or manually concluded tasks are rejected." permission:"idc:admin:product-fulfillment:recover"`
	ProductFulfillmentManualActionReq
	RecoveryConfiguration *provisionresourcecap.RecoveryConfiguration `json:"recoveryConfiguration,omitempty" dc:"Operational upstream correction preserving all paid specifications and amounts; omitted retains the original paid configuration" eg:"{}"`
}

// ProductFulfillmentTaskRecoveryRes returns the committed workflow projection.
type ProductFulfillmentTaskRecoveryRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Current fulfillment state after the atomic recovery command" eg:"{}"`
}
