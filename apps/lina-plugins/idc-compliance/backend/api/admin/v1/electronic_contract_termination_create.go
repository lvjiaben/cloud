// This file defines the management command that submits a reviewed contract termination.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTerminationCreateReq submits one reviewed Tencent release agreement.
type ElectronicContractTerminationCreateReq struct {
	g.Meta                    `path:"/admin/compliance/electronic-contracts/{contractNo}/terminations" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Terminate a completed electronic contract" dc:"Submit one release agreement using the original contract signers after the administrator reviews the content and confirms a second time. No current-session check is required, and no client-supplied effective time is accepted." permission:"idc:admin:compliance:contract:terminate"`
	ContractNo                string `json:"contractNo" in:"path" v:"required|max-length:64" dc:"Exact completed contract to terminate" eg:"CTR_0123456789ABCDEF"`
	ExpectedVersion           int64  `json:"expectedVersion" v:"required|min:1" dc:"Completed contract version shown during review" eg:"3"`
	Reason                    string `json:"reason" v:"required|max-length:200" dc:"Required release reason reviewed by the administrator" eg:"Mutual agreement to terminate"`
	RemainInForceItem         string `json:"remainInForceItem" v:"max-length:200" dc:"Optional clauses that remain in force" eg:"The confidentiality clause remains effective"`
	OriginalExpenseSettlement string `json:"originalExpenseSettlement" v:"max-length:200" dc:"Optional expense settlement arrangement" eg:"No outstanding expenses remain"`
	OriginalOtherSettlement   string `json:"originalOtherSettlement" v:"max-length:200" dc:"Optional settlement of other original-contract matters" eg:"No other pending matters remain"`
	OtherDeals                string `json:"otherDeals" v:"max-length:200" dc:"Optional other release agreement terms" eg:"Both parties retain archived copies"`
	Confirmed                 bool   `json:"confirmed" v:"required|in:true" dc:"True only after the administrator reviews the full release content and confirms submission a second time" eg:"true"`
	IdempotencyKey            string `json:"idempotencyKey" v:"required|length:16,128" dc:"Retry key bound to this exact termination submission" eg:"contract-termination-20260817-0001"`
}

// ElectronicContractTerminationCreateRes returns no Tencent identifier or effective time.
type ElectronicContractTerminationCreateRes struct {
	Item *ElectronicContractSigningSubmissionItem `json:"item" dc:"Safe installation-local termination signing result" eg:"{}"`
}
