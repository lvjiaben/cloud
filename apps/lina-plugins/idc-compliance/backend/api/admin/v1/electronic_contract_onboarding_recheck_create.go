// This file defines explicit Tencent onboarding-state rechecks.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractOnboardingRecheckCreateReq carries only a client retry key.
type ElectronicContractOnboardingRecheckCreateReq struct {
	g.Meta         `path:"/admin/compliance/electronic-contract/onboarding/rechecks" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Recheck Tencent onboarding state" dc:"Actively query Tencent through the installation-local gateway and commit only a verified local opaque binding." permission:"idc:admin:compliance:supplier:subject:bind"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,128" dc:"Client-generated retry key" eg:"tencent-recheck-01234567-89ab-cdef-0123-456789abcdef"`
}

// ElectronicContractOnboardingRecheckCreateRes returns the refreshed secret-free state.
type ElectronicContractOnboardingRecheckCreateRes struct {
	Item *ElectronicContractOnboardingItem `json:"item" dc:"Onboarding projection refreshed from Tencent without returning or opening another URL"`
}
