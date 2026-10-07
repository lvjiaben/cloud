package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerActivationRedeemReq struct {
	g.Meta `path:"/account/activation-invitations/redeem" method:"post" tags:"IDC Customer Activation" summary:"Redeem my customer activation invitation" dc:"Authenticated narrow exception for an identity without any existing customer or staff audience. It can only bind the pending customer named by the single-use invitation."`
	Token  string `json:"token" v:"required|min-length:32|max-length:128" dc:"Single-use activation token received through the configured notification channel"`
}

type CustomerActivationRedeemRes struct {
	CustomerNo   string `json:"customerNo"`
	CustomerType string `json:"customerType"`
	Status       string `json:"status"`
	Reused       bool   `json:"reused"`
}

// CustomerInitialPasswordStatusReq reads the current direct-created login gate.
type CustomerInitialPasswordStatusReq struct {
	g.Meta `path:"/account/initial-password/status" method:"get" tags:"IDC Customer Activation" summary:"Read my initial password change status" dc:"Returns only whether the current direct-created customer must replace the administrator-provided initial password."`
}

// CustomerInitialPasswordStatusRes returns the current password-change gate.
type CustomerInitialPasswordStatusRes struct {
	Required bool `json:"required" dc:"Whether password replacement is required before account use" eg:"true"`
}

// CustomerInitialPasswordChangeReq replaces the current initial password.
type CustomerInitialPasswordChangeReq struct {
	g.Meta      `path:"/account/initial-password" method:"put" tags:"IDC Customer Activation" summary:"Replace my initial customer password" dc:"Replaces the password of the current direct-created customer and permanently clears the first-login gate after the host accepts the new password."`
	NewPassword string `json:"newPassword" v:"required|min-length:8|max-length:128" dc:"New customer password" eg:"NewStrongPassw0rd!"`
}

// CustomerInitialPasswordChangeRes confirms that the gate was cleared.
type CustomerInitialPasswordChangeRes struct {
	Required bool `json:"required" dc:"Always false after a successful replacement" eg:"false"`
}
