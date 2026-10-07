// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Challenge is the golang structure of table plugin_shared_services_challenge for DAO operations like Where/Data.
type Challenge struct {
	g.Meta           `orm:"table:plugin_shared_services_challenge, do:true"`
	Id               any        //
	TenantId         any        //
	ChallengeNo      any        //
	CallerPluginId   any        //
	Purpose          any        //
	BindingDigest    any        //
	ProviderCode     any        //
	PresentationKind any        //
	AnswerSalt       any        //
	AnswerDigest     any        //
	ProofDigest      any        //
	Status           any        //
	AttemptCount     any        //
	MaxAttempts      any        //
	ExpiresAt        *time.Time //
	ProofExpiresAt   any        //
	VerifiedAt       any        //
	ConsumedAt       any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
