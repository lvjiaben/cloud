// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContactVerification is the golang structure of table plugin_idc_notify_contact_verification for DAO operations like Where/Data.
type ContactVerification struct {
	g.Meta                  `orm:"table:plugin_idc_notify_contact_verification, do:true"`
	TenantId                any        //
	VerificationNo          any        //
	PurposeCode             any        //
	OperationCode           any        //
	DestinationKind         any        //
	DestinationDigest       any        //
	BrowserDigest           any        //
	TicketDigest            any        //
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
	RetryAfterAt            *time.Time //
	ExpiresAt               *time.Time //
	ProviderBoundaryCrossed any        //
	VerifiedAt              *time.Time //
	CreatedAt               *time.Time //
	UpdatedAt               *time.Time //
}
