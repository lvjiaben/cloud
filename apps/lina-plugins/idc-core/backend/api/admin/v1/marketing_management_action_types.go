// This file contains bounded staff marketing-action response DTOs.

package v1

type MarketingManagedAssetMutation struct {
	AssetNo    string `json:"assetNo" dc:"Stable promotion asset business number" eg:"mast_00112233445566778899aabb"`
	CustomerNo string `json:"customerNo" dc:"Owning customer business number" eg:"customer-001"`
	Status     string `json:"status" dc:"Resulting asset lifecycle status" eg:"active"`
	UpdatedAt  int64  `json:"updatedAt" dc:"Resulting update time in Unix milliseconds" eg:"1767225600000"`
	Reused     bool   `json:"reused" dc:"Whether the same idempotent request was replayed" eg:"false"`
}

type MarketingManagedPointsMutation struct {
	LotNo           string `json:"lotNo" dc:"Affected points lot business number" eg:"plot_0011223344556677"`
	CustomerNo      string `json:"customerNo" dc:"Owning customer business number" eg:"customer-001"`
	AvailablePoints int64  `json:"availablePoints" dc:"Resulting available non-monetary points" eg:"1200"`
	FrozenPoints    int64  `json:"frozenPoints" dc:"Resulting frozen non-monetary points" eg:"100"`
	DebtPoints      int64  `json:"debtPoints" dc:"Resulting outstanding points debt" eg:"0"`
	Reused          bool   `json:"reused" dc:"Whether the same idempotent request was replayed" eg:"false"`
}
