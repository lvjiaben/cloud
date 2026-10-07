// This file defines the customer-owned authoritative red-letter recheck API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerRedLetterAuthoritativeStateUpdateReq struct {
	g.Meta          `path:"/account/compliance/red-letter-tasks/{taskNo}/authoritative-state" method:"put" tags:"IDC Compliance Account" summary:"Recheck my red-letter confirmation" dc:"Ask the verified supplier adapter for authoritative state. Returning from a supplier page or clicking this action never confirms the task by itself."`
	TaskNo          string `json:"taskNo" v:"required|max-length:64" dc:"Customer-owned red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed task version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"red-letter-recheck-0001"`
}

type CustomerRedLetterAuthoritativeStateUpdateRes struct {
	Item *CustomerRedLetterTaskItem `json:"item" dc:"Customer-safe state after authoritative recheck" eg:"{}"`
}
