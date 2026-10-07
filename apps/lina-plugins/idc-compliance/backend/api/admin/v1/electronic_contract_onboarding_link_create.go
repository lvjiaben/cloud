// This file defines official Tencent onboarding-link creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

// ElectronicContractOnboardingLinkCreateReq carries only a client retry key.
type ElectronicContractOnboardingLinkCreateReq struct {
	g.Meta         `path:"/admin/compliance/electronic-contract/onboarding/links" method:"post" tags:"IDC Compliance Electronic Contract" summary:"Create official Tencent onboarding link" dc:"Create one official short-lived Tencent console link on the installation-local gateway. Sensitive certification details remain on Tencent's page." permission:"idc:admin:compliance:supplier:subject:bind"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|length:8,128" dc:"Client-generated retry key" eg:"tencent-onboarding-01234567-89ab-cdef-0123-456789abcdef"`
}

// ElectronicContractOnboardingLinkCreateRes returns the official short-lived URL when created.
type ElectronicContractOnboardingLinkCreateRes struct {
	Item *ElectronicContractOnboardingItem `json:"item" dc:"Onboarding projection containing the newly created official Tencent URL when successful"`
}
