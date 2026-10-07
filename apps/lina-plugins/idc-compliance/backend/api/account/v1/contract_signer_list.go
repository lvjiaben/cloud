package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractSignerListReq struct {
	g.Meta `path:"/account/compliance/contract-signers" method:"get" tags:"IDC Compliance Account" summary:"List enterprise contract signers" dc:"List only the authenticated enterprise customer's separate contract-signer directory."`
}
type ContractSignerListRes struct {
	Items []ContractSignerItem `json:"items"`
}
