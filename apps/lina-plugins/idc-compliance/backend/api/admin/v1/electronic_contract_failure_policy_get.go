// This file defines the read-only API for the active contract-failure policy.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractFailurePolicyGetReq struct {
	g.Meta `path:"/admin/compliance/electronic-contract/failure-policy" method:"get" tags:"IDC Compliance Electronic Contract" summary:"Get contract failure handling policy" dc:"Return the active installation-local handling period. Existing failed contracts keep the version and deadline frozen when their terminal event was received." permission:"idc:admin:compliance:contract:query"`
}

type ElectronicContractFailurePolicyGetRes struct {
	Item *ElectronicContractFailurePolicyItem `json:"item" dc:"Active installation-local contract failure policy" eg:"{}"`
}
