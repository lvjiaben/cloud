// This file defines the secret-free Tencent onboarding read contract.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractOnboardingGetReq selects the current installation's onboarding state.
type ElectronicContractOnboardingGetReq struct {
	g.Meta `path:"/admin/compliance/electronic-contract/onboarding" method:"get" tags:"IDC Compliance Electronic Contract" summary:"Get Tencent electronic contract onboarding state" dc:"Return the existing merchant profile and safe installation-local Tencent activation projection without credentials or supplier subject identifiers." permission:"idc:admin:compliance:supplier:query"`
}

// ElectronicContractOnboardingGetRes returns one secret-free onboarding projection.
type ElectronicContractOnboardingGetRes struct {
	Item *ElectronicContractOnboardingItem `json:"item" dc:"Secret-free Tencent electronic contract onboarding projection"`
}
