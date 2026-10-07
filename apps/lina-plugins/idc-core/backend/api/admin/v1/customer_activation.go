package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerActivationInvitationCreateReq struct {
	g.Meta               `path:"/admin/customers/activation-invitations" method:"post" tags:"IDC Customer Management" summary:"Create a pending customer and send an activation invitation" dc:"Creates only a pending customer and a 24-hour single-use invitation. Administrators never set or receive a customer password." permission:"idc:admin:customer:create"`
	CustomerType         string `json:"customerType" v:"required|in:individual" dc:"All new accounts start as individual customers"`
	DisplayName          string `json:"displayName" v:"required|max-length:256" dc:"Unverified customer display name"`
	CountryCode          string `json:"countryCode" v:"required|min-length:2|max-length:2" dc:"Two-letter customer country code"`
	PreferredLocale      string `json:"preferredLocale" v:"required|in:zh-CN,en-US" dc:"Customer interface language"`
	Email                string `json:"email" v:"max-length:320" dc:"Exactly one invitation destination: email or phone"`
	PhoneCountryCode     string `json:"phoneCountryCode" v:"max-length:8" dc:"Calling code when phone is used"`
	Phone                string `json:"phone" v:"max-length:32" dc:"Phone number when SMS is used"`
	BindExistingIdentity bool   `json:"bindExistingIdentity" dc:"Resolve the invitation email or phone to one existing visible host identity and require that same identity to redeem"`
	Reason               string `json:"reason" v:"required|min-length:10|max-length:512" dc:"Audited reason for creating or binding the customer"`
	IdempotencyKey       string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Browser-generated retry key"`
}

type CustomerActivationInvitationCreateRes struct {
	InvitationNo          string `json:"invitationNo"`
	CustomerNo            string `json:"customerNo"`
	CustomerType          string `json:"customerType"`
	DisplayName           string `json:"displayName"`
	DestinationHint       string `json:"destinationHint"`
	ExistingIdentityBound bool   `json:"existingIdentityBound"`
	Status                string `json:"status"`
	ExpiresAt             int64  `json:"expiresAt" dc:"Authoritative expiry as Unix milliseconds"`
	Reused                bool   `json:"reused"`
}

type CustomerActivationInvitationResendReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/activation-invitations/resend" method:"post" tags:"IDC Customer Management" summary:"Invalidate the old invitation and send a new customer activation invitation" permission:"idc:admin:customer:create"`
	CustomerNo     string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Pending customer number"`
	Reason         string `json:"reason" v:"required|min-length:10|max-length:512" dc:"Audited reason for resending the invitation"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Browser-generated retry key"`
}

type CustomerActivationInvitationResendRes = CustomerActivationInvitationCreateRes

// CustomerDirectAccountCreateReq creates an active customer login without an invitation.
type CustomerDirectAccountCreateReq struct {
	g.Meta           `path:"/admin/customers/direct-accounts" method:"post" tags:"IDC Customer Management" summary:"Create an active customer login directly" dc:"Creates one active customer identity from an administrator-provided phone or email login and initial password. No activation message is sent." permission:"idc:admin:customer:create"`
	CustomerType     string `json:"customerType" v:"required|in:individual" dc:"All new accounts start as individual customers" eg:"individual"`
	DisplayName      string `json:"displayName" v:"required|max-length:256" dc:"Customer display name" eg:"Taylor Chen"`
	CountryCode      string `json:"countryCode" v:"required|min-length:2|max-length:2" dc:"Two-letter customer country or region code" eg:"CN"`
	PreferredLocale  string `json:"preferredLocale" v:"required|in:zh-CN,en-US" dc:"Customer interface language" eg:"zh-CN"`
	Email            string `json:"email" v:"max-length:320" dc:"Email login; exactly one of email or phone is required" eg:"customer@example.com"`
	PhoneCountryCode string `json:"phoneCountryCode" v:"max-length:8" dc:"International calling code when phone login is used" eg:"+86"`
	Phone            string `json:"phone" v:"max-length:32" dc:"Phone login without the calling code" eg:"13800138000"`
	InitialPassword  string `json:"initialPassword" v:"required|min-length:8|max-length:128" dc:"One-time initial password that is hashed only by the host user owner and never returned" eg:"StrongPassw0rd!"`
	Reason           string `json:"reason" v:"required|min-length:10|max-length:512" dc:"Audited reason for directly creating the customer" eg:"The customer confirmed account creation through an offline contract."`
	IdempotencyKey   string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Browser-generated retry key" eg:"customer-direct-550e8400-e29b-41d4-a716-446655440000"`
}

// CustomerDirectAccountCreateRes returns only non-secret creation facts.
type CustomerDirectAccountCreateRes struct {
	CustomerNo string `json:"customerNo" dc:"Created customer business number" eg:"CUS_0123456789abcdef0123456789abcdef"`
	UserID     int    `json:"userId" dc:"Created host login identity number" eg:"101"`
	Status     string `json:"status" dc:"Customer status: active" eg:"active"`
	Reused     bool   `json:"reused" dc:"Whether the same idempotent request was replayed" eg:"false"`
}

type CustomerPasswordResetReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/password" method:"put" tags:"IDC Customer Management" summary:"Reset an active customer login password" dc:"Replaces the password through the host user owner and requires the customer to choose a private password after the next login." permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Opaque internal customer number"`
	NewPassword    string `json:"newPassword" v:"required|min-length:8|max-length:128" dc:"Temporary password that is never returned or audited"`
	Reason         string `json:"reason" v:"required|min-length:10|max-length:512" dc:"Audited reason for resetting the password"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Browser-generated operation key"`
}

type CustomerPasswordResetRes struct{}
