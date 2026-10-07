// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerOnboardingIntent is the golang structure of table plugin_idc_core_customer_onboarding_intent for DAO operations like Where/Data.
type CustomerOnboardingIntent struct {
	g.Meta                 `orm:"table:plugin_idc_core_customer_onboarding_intent, do:true"`
	Id                     any        //
	TenantId               any        //
	IntentNo               any        // Opaque onboarding intent business number
	TokenDigest            any        // SHA-256 digest of the single-use opaque token
	BrowserNonceDigest     any        // SHA-256 digest of the browser-bound nonce
	NormalizedUsernameHash any        // Digest of the normalized requested username
	ContactChannelCode     any        //
	ContactHash            any        // Digest of the verified contact destination
	CustomerTypeCode       any        //
	StatusCode             any        //
	ReturnPath             any        // Validated same-origin path used after onboarding
	HostRegistrationUserId any        // Host identity returned by native registration
	ConsumedByUserId       any        //
	ResultCustomerNo       any        // Customer business number created by successful redemption
	VerifiedContactAt      *time.Time //
	ExpiresAt              *time.Time // Intent expiry time
	ConsumedAt             *time.Time //
	AttemptCount           any        //
	Version                any        //
	CreatedAt              *time.Time //
	UpdatedAt              *time.Time //
	OperationDigest        any        //
	RequestDigest          any        //
}
