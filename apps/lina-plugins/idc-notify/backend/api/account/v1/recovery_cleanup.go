// This file defines bounded self-only password recovery session cleanup.
package v1

import "github.com/gogf/gf/v2/frame/g"

// RecoverySessionCleanupReq retries a completed reset's outstanding cleanup.
type RecoverySessionCleanupReq struct {
	g.Meta     `path:"/account/password-reset/session-cleanups" method:"post" tags:"IDC Customer Authentication" summary:"Revoke my older password-reset sessions" dc:"Uses only the authenticated customer's visible sessions; does nothing without a pending completed reset."`
	RecoveryNo string `json:"recoveryNo" v:"max-length:48" dc:"Optional exact completed reset to clean; empty retries the authenticated account latest reset" eg:"nver_01"`
}

// RecoverySessionCleanupRes preserves pending cleanup as a distinct state.
type RecoverySessionCleanupRes struct {
	Pending bool `json:"pending" dc:"Whether another cleanup attempt is needed" eg:"false"`
}
