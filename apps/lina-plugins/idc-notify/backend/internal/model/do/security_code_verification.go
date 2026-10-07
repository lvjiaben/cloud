// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SecurityCodeVerification is the golang structure of table plugin_idc_notify_security_code_verification for DAO operations like Where/Data.
type SecurityCodeVerification struct {
	g.Meta                  `orm:"table:plugin_idc_notify_security_code_verification, do:true"`
	TenantId                any        //
	VerificationNo          any        //
	CallerPluginId          any        //
	PurposeCode             any        //
	PurposeVersion          any        //
	SubjectDigest           any        //
	DestinationDigest       any        //
	BrowserDigest           any        //
	LocaleCode              any        //
	TemplateNo              any        //
	TemplateVersion         any        //
	RequestShapeDigest      any        //
	IdempotencyDigest       any        //
	CodeDigest              any        //
	MaskedDestination       any        //
	StatusCode              any        //
	DeliveryState           any        //
	FailedAttempts          any        //
	VerificationVersion     any        //
	RetryAfterAt            *time.Time //
	ExpiresAt               *time.Time //
	ProviderBoundaryCrossed any        //
	VerifiedAt              *time.Time //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
