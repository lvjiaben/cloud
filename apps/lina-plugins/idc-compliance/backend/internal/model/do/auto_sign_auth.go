// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// AutoSignAuth is the golang structure of table plugin_idc_compliance_auto_sign_auth for DAO operations like Where/Data.
type AutoSignAuth struct {
	g.Meta                      `orm:"table:plugin_idc_compliance_auto_sign_auth, do:true"`
	Id                          any        //
	TenantId                    any        //
	AuthorizationNo             any        //
	ProviderCode                any        //
	CurrencyCode                any        //
	AllowRequiredOrder          any        //
	AllowManualOptional         any        //
	MaxAmountMinor              any        //
	SuccessLimit                any        //
	SuccessCount                any        //
	StatusCode                  any        //
	ExpiresAt                   *time.Time //
	CreatedByUserId             any        //
	RevokedByUserId             *int       //
	RevokedAt                   *time.Time //
	CreationIdempotencyDigest   any        //
	CreationFingerprint         any        //
	RevocationIdempotencyDigest *string    //
	RevocationFingerprint       *string    //
	Version                     any        //
	CreatedAt                   *time.Time //
	UpdatedAt                   *time.Time //
}
