// This file defines creation of a short-lived signing link for an existing flow.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerContractSigningLinkCreateReq requests a fresh link for the existing flow.
type CustomerContractSigningLinkCreateReq struct {
	g.Meta     `path:"/account/compliance/contracts/{contractNo}/signing-links" method:"post" tags:"IDC Compliance Account" summary:"Create my contract signing link" dc:"Create one fresh 30-minute Tencent H5 link only for the authenticated customer's existing signable contract and existing Tencent flow. This never creates a second contract, relaunches a flow, or extends the seven-day contract deadline."`
	ContractNo string `json:"contractNo" v:"required|max-length:64" dc:"Customer-owned contract number" eg:"ctr_0123456789abcdef0123456789abcdef"`
}

// CustomerContractSigningLinkCreateRes returns a non-persisted link and current safe state.
type CustomerContractSigningLinkCreateRes struct {
	Item CustomerSigningLinkItem `json:"item" dc:"Fresh non-persisted Tencent H5 signing link and current safe contract state" eg:"{}"`
}
