package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerOperationPolicy struct {
	RequireHighRiskReauth bool   `json:"requireHighRiskReauth" dc:"Whether dangerous customer server operations require a current-session confirmation"`
	AllowCustomerTransfer bool   `json:"allowCustomerTransfer" dc:"Whether customers may request bilateral server ownership transfers"`
	Version               int64  `json:"version" dc:"Optimistic concurrency version"`
	UpdatedAt             string `json:"updatedAt" dc:"Last committed policy update time"`
}

type ServerOperationPolicyGetReq struct {
	g.Meta `path:"/admin/resource/server-operation-policy" method:"get" tags:"IDC Server Operations" summary:"Get server operation security policy" permission:"idc:admin:resource:server-operation-policy:read"`
}

type ServerOperationPolicyGetRes struct {
	Policy *ServerOperationPolicy `json:"policy"`
}

type ServerOperationPolicyUpdateReq struct {
	g.Meta                `path:"/admin/resource/server-operation-policy" method:"put" tags:"IDC Server Operations" summary:"Update server operation security policy" permission:"idc:admin:resource:server-operation-policy:write"`
	RequireHighRiskReauth bool  `json:"requireHighRiskReauth"`
	AllowCustomerTransfer bool  `json:"allowCustomerTransfer"`
	ExpectedVersion       int64 `json:"expectedVersion" v:"required|min:1"`
}

type ServerOperationPolicyUpdateRes struct {
	Policy *ServerOperationPolicy `json:"policy"`
}
