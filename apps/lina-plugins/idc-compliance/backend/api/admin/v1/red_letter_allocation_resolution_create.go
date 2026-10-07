// This file defines the staff red-letter allocation resolution API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RedLetterAllocationResolutionCreateReq struct {
	g.Meta          `path:"/admin/compliance/red-letter-tasks/{taskNo}/allocation-resolutions" method:"post" tags:"IDC Compliance Red Letter" summary:"Resolve red-letter invoice allocations" dc:"Select the complete eligible original-invoice allocation plan when a refund cannot be mapped uniquely. The server rejects guessed, incomplete, excessive, or stale plans." permission:"idc:admin:compliance:red-letter:handle"`
	TaskNo          string                         `json:"taskNo" v:"required|max-length:64" dc:"Stable red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
	ExpectedVersion int64                          `json:"expectedVersion" v:"required|min:1" dc:"Last observed task version" eg:"1"`
	IdempotencyKey  string                         `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"red-letter-allocation-0001"`
	Selections      []RedLetterAllocationSelection `json:"selections" v:"required" dc:"Complete explicit original-invoice allocation plan of one to two hundred selections; collection bounds are enforced by the red-letter service" eg:"[]"`
}

type RedLetterAllocationResolutionCreateRes struct {
	Item *RedLetterTaskItem `json:"item" dc:"Task after exact allocation resolution" eg:"{}"`
}
