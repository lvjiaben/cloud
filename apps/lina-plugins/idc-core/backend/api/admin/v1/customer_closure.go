package v1

import "github.com/gogf/gf/v2/frame/g"

type ClosureCompleteReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/closure-requests/{requestNo}/completion" method:"post" tags:"IDC Customer Management" summary:"Approve and complete customer closure" permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" in:"path" v:"required|max-length:40"`
	RequestNo      string `json:"requestNo" in:"path" v:"required|max-length:40"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128"`
}

type ClosureCompleteRes struct {
	RequestNo   string `json:"requestNo"`
	CustomerNo  string `json:"customerNo"`
	Status      string `json:"status"`
	CompletedAt int64  `json:"completedAt"`
}
