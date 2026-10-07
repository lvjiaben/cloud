// This file defines the customer-owned red-letter task detail API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerRedLetterTaskGetReq struct {
	g.Meta `path:"/account/compliance/red-letter-tasks/{taskNo}" method:"get" tags:"IDC Compliance Account" summary:"Get my red-letter task" dc:"Return one task only when it belongs to the authenticated customer. Staff-only reasons and private evidence are excluded."`
	TaskNo string `json:"taskNo" v:"required|max-length:64" dc:"Customer-owned red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
}

type CustomerRedLetterTaskGetRes struct {
	Item *CustomerRedLetterTaskItem `json:"item" dc:"Customer-safe red-letter task" eg:"{}"`
}
