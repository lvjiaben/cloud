// This file declares an explicit staff fact for unknown provisioning results.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProductFulfillmentNonAcceptanceVerificationReq never initiates provider work.
type ProductFulfillmentNonAcceptanceVerificationReq struct {
	g.Meta              `path:"/admin/product-fulfillment/tasks/{intentNo}/non-acceptance-verifications" method:"post" tags:"IDC Product Fulfillment" summary:"Record verified provisioning non-acceptance" dc:"Record the authorized administrator assertion that upstream has neither an instance nor a task. Preserve unknown history; enable separate recovery or financial refund checks. This action never calls upstream." permission:"idc:admin:product-fulfillment:recover"`
	IntentNo            string `json:"intentNo" in:"path" v:"required|max-length:40" dc:"Stable fulfillment intent number" eg:"pft_123"`
	ExpectedVersion     int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic workflow version" eg:"3"`
	IdempotencyKey      string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Action key reused unchanged across uncertain retries" eg:"verification-00000001"`
	Note                string `json:"note" v:"required|length:10,512" dc:"Required processing note of 10 to 512 characters" eg:"Checked upstream: no instance or task exists."`
	ConfirmedNoCreation bool   `json:"confirmedNoCreation" v:"required|in:true" dc:"Explicit administrator confirmation after checking upstream that no instance or task was created; must be true" eg:"true"`
}

// ProductFulfillmentNonAcceptanceVerificationRes projects the committed staff fact.
type ProductFulfillmentNonAcceptanceVerificationRes struct {
	Task *ProductFulfillmentTaskDetail `json:"task" dc:"Current fulfillment state after independent non-acceptance verification" eg:"{}"`
}

// ProductFulfillmentNonAcceptanceRecord preserves a separate operator fact.
type ProductFulfillmentNonAcceptanceRecord struct {
	ProviderTaskNo string `json:"providerTaskNo" dc:"Original resource task number whose unknown history remains intact" eg:"task_123"`
	ActorUserID    int64  `json:"actorUserId" dc:"Administrator who verified non-acceptance" eg:"1"`
	Note           string `json:"note" dc:"Administrator verification note; 10 to 512 characters" eg:"Checked upstream: no instance and no task exist."`
	VerifiedAt     int64  `json:"verifiedAt" dc:"Verification time as Unix timestamp in milliseconds" eg:"1792483200000"`
}
