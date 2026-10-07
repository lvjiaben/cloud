// This file defines the explicit Tencent template synchronization action.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractTemplateTencentSyncCreateReq carries no browser-owned supplier identifiers.
type ElectronicContractTemplateTencentSyncCreateReq struct {
	g.Meta `path:"/admin/compliance/electronic-contract/templates/tencent-syncs" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Synchronize Tencent electronic contract templates" dc:"Read one complete template snapshot through the installation-local gateway only after this explicit management action. Failure preserves the existing local projection." permission:"idc:admin:compliance:contract-template:manage"`
}

// ElectronicContractTemplateTencentSyncCreateRes summarizes the committed snapshot.
type ElectronicContractTemplateTencentSyncCreateRes struct {
	Item *ElectronicContractTemplateSyncResult `json:"item" dc:"Committed complete-snapshot summary" eg:"{}"`
}
