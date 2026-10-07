// This file defines the versioned contract-failure policy update API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ElectronicContractFailurePolicyUpdateReq struct {
	g.Meta          `path:"/admin/compliance/electronic-contract/failure-policy" method:"put" tags:"IDC Compliance Electronic Contract" summary:"Update contract failure handling policy" dc:"Create a new 0-30 natural-day policy revision. The change affects only future terminal contract events and never changes an already frozen deadline." permission:"idc:admin:compliance:contract:failure-policy"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Active policy version currently shown" eg:"1"`
	HandlingDays    int    `json:"handlingDays" v:"min:0|max:30" dc:"Natural-day period; zero makes the next terminal failure immediately due" eg:"3"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|length:16,128" dc:"Retry key bound to this exact policy update" eg:"contract-failure-policy-20260818-0001"`
}

type ElectronicContractFailurePolicyUpdateRes struct {
	Item *ElectronicContractFailurePolicyItem `json:"item" dc:"Created contract failure policy revision" eg:"{}"`
}
