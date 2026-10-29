// This file declares an authorized retry of local management binding only.
// A request never changes financial facts or reissues supplier provisioning.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProductFulfillmentManagementRepairReq targets a stopped local notification.
type ProductFulfillmentManagementRepairReq struct {
	g.Meta          `path:"/admin/product-fulfillment/tasks/{intentNo}/management-repairs" method:"post" tags:"IDC Product Fulfillment" summary:"Repair local service management linking" dc:"Requeue the original resource-owner local management notification for a paid, completed delivery. Recheck customer scope, current version, service ownership, stopped delivery, leases and refunds. Never creates supplier tasks, changes money or replays provisioning." permission:"idc:admin:product-fulfillment:recover"`
	IntentNo        string `json:"intentNo" in:"path" v:"required|max-length:40" dc:"Existing customer-scoped fulfillment intent number" eg:"pft_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected current workflow version; local management repair does not increment the business workflow version" eg:"3"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|length:16,128" dc:"Reuse the same operation key across uncertain replies; a changed request with the same key is rejected" eg:"local-management-0123456789abcdef"`
}

// ProductFulfillmentManagementRepairRes keeps the original delivery projection.
type ProductFulfillmentManagementRepairRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Current fulfillment and local management progress after the atomic repair; original business and financial outcomes stay unchanged" eg:"{}"`
}
