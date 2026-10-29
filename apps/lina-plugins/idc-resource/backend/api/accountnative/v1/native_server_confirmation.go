// This file defines current-session confirmation for exact native operations.
package v1

import (
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
)

// NativeServerConfirmationReq prepares impact confirmation, never a cloud write.
type NativeServerConfirmationReq struct {
	g.Meta         `path:"/account/native-servers/{serverNo}/operation-confirmations" method:"post" tags:"IDC Account Servers" summary:"Prepare my server operation confirmation" dc:"Check current ownership, entitlement, capability and normalized inputs before issuing the core-owned single-use current-session confirmation. No supplier write is performed."`
	ServerNo       string          `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public owned server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Action         string          `json:"action" v:"required|max-length:64" dc:"Requested native management action" eg:"reinstall"`
	Parameters     json.RawMessage `json:"parameters" dc:"Exact whitelisted non-secret parameters for this operation" eg:"{\"os\":\"101\"}"`
	IdempotencyKey string          `json:"idempotencyKey" v:"required|length:1,128|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]*$" dc:"Request key retained for this exact operation" eg:"native-operation-1"`
}

// NativeServerConfirmationRes is not an authorization to bypass resource gates.
type NativeServerConfirmationRes struct {
	Required   bool   `json:"required" dc:"The current merchant policy requires the returned single-use confirmation" eg:"true"`
	ProofToken string `json:"proofToken" dc:"Opaque current-session proof; never cache or log this value" eg:""`
	ExpiresAt  *int64 `json:"expiresAt,omitempty" dc:"Proof expiry as Unix timestamp in milliseconds; absent when no proof is required" eg:"1792382700000"`
}
