// This file defines the explicit paid-order cancellation action after a
// required contract reaches a supplier-authoritative terminal state.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractOrderCancellationCreateReq struct {
	g.Meta     `path:"/admin/compliance/electronic-contracts/{contractNo}/order-cancellation" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Cancel a paid order after required contract failure" dc:"Immediately stop untouched fulfillment, release inventory, and create one refund handoff for the terminal required contract. The response never claims that money has already returned." permission:"idc:admin:compliance:contract:cancel-order"`
	ContractNo string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Terminal required contract number" eg:"CTR_0123456789ABCDEF"`
	Confirmed  bool   `json:"confirmed" v:"required|in:true" dc:"Administrator explicitly confirmed immediate order cancellation" eg:"true"`
}

type ElectronicContractOrderCancellationCreateRes struct {
	Item *ElectronicContractFailureCompensationItem `json:"item" dc:"Safe order stop and refund handoff result" eg:"{}"`
}
