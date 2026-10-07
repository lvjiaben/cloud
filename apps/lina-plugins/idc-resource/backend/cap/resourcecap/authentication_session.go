package resourcecap

import (
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// MaximumAuthenticationTokenBytes bounds one encrypted upstream login token.
	MaximumAuthenticationTokenBytes = 8 << 10
	minimumAuthenticationSessionTTL = time.Minute
	maximumAuthenticationSessionTTL = time.Hour
)

// Validate verifies a channel-scoped login-cache request.
func (r AuthenticationSessionRequest) Validate() error {
	if err := ValidateChannelRef(r.ChannelRef); err != nil {
		return err
	}
	if r.TTL < minimumAuthenticationSessionTTL || r.TTL > maximumAuthenticationSessionTTL {
		return gerror.New("resource Provider authentication session TTL is invalid")
	}
	return nil
}

// Validate verifies a generation-fenced invalidation request.
func (r AuthenticationSessionInvalidation) Validate() error {
	if err := ValidateChannelRef(r.ChannelRef); err != nil {
		return err
	}
	if r.Generation <= 0 {
		return gerror.New("resource Provider authentication generation is invalid")
	}
	return nil
}
