// This file defines the customer's explicit supplier-authoritative contract recheck API.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerContractAuthoritativeStateUpdateReq performs one supplier-authoritative read.
type CustomerContractAuthoritativeStateUpdateReq struct {
	g.Meta     `path:"/account/compliance/contracts/{contractNo}/authoritative-state" method:"put" tags:"IDC Compliance Account" summary:"Recheck my contract signing state" dc:"Query Tencent through the installation-local compliance gateway and converge only an authoritative read result. Browser return parameters never determine contract completion."`
	ContractNo string `json:"contractNo" v:"required|max-length:64" dc:"Customer-owned contract number" eg:"ctr_0123456789abcdef0123456789abcdef"`
}

// CustomerContractAuthoritativeStateUpdateRes returns the safe state after the authoritative read.
type CustomerContractAuthoritativeStateUpdateRes struct {
	Item CustomerContractItem `json:"item" dc:"Customer-safe contract state after the authoritative read" eg:"{}"`
}
