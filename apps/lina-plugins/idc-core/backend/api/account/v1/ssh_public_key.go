// This file defines customer-owned SSH public-key HTTP contracts.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SSHPublicKeyItem is the safe projection returned to the customer.
type SSHPublicKeyItem struct {
	KeyNo       string `json:"keyNo" dc:"Stable owned SSH public-key number" eg:"SSH_0123456789abcdef0123456789abcdef"`
	Name        string `json:"name" dc:"Customer-provided key name" eg:"Work laptop"`
	Algorithm   string `json:"algorithm" dc:"Parsed SSH public-key algorithm" eg:"ssh-ed25519"`
	Fingerprint string `json:"fingerprint" dc:"OpenSSH SHA256 public-key fingerprint" eg:"SHA256:3q2+7w..."`
	Status      string `json:"status" dc:"Key status: active or revoked" eg:"active"`
	Version     int64  `json:"version" dc:"Current optimistic-lock version" eg:"1"`
	CreatedAt   int64  `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1787184000000"`
	RevokedAt   *int64 `json:"revokedAt" dc:"Revocation time as a Unix timestamp in milliseconds, or null" eg:"1787187600000"`
}

// SSHPublicKeyListReq lists the current customer's keys without key material.
type SSHPublicKeyListReq struct {
	g.Meta `path:"/account/ssh-public-keys" method:"get" tags:"IDC Customer SSH Keys" summary:"List my SSH public keys" dc:"Return at most one hundred named SSH public-key fingerprints owned by the current customer. Public-key material is not returned."`
}

// SSHPublicKeyListRes returns a bounded key list.
type SSHPublicKeyListRes struct {
	Items []*SSHPublicKeyItem `json:"items" dc:"Current customer's SSH public-key projections" eg:"[]"`
}

// SSHPublicKeyCreateReq strictly parses and stores one public key.
type SSHPublicKeyCreateReq struct {
	g.Meta        `path:"/account/ssh-public-keys" method:"post" tags:"IDC Customer SSH Keys" summary:"Save my SSH public key" dc:"Strictly parse and save one plain OpenSSH authorized public-key line. Private keys, authorization options, certificates, and multiple keys are rejected."`
	Name          string `json:"name" v:"required|max-length:80" dc:"Name used to recognize this public key" eg:"Work laptop"`
	AuthorizedKey string `json:"authorizedKey" v:"required|max-length:16384" dc:"One plain OpenSSH authorized public-key line; never a private key" eg:"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... work-laptop"`
	RequestNo     string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Client-generated retry key reused only for this exact create request" eg:"ssh-key-550e8400-e29b-41d4-a716-446655440000"`
}

// SSHPublicKeyCreateRes returns the saved fingerprint without key material.
type SSHPublicKeyCreateRes struct {
	Item *SSHPublicKeyItem `json:"item" dc:"Saved SSH public-key projection" eg:"{}"`
}

// SSHPublicKeyRevokeReq revokes one active owned key.
type SSHPublicKeyRevokeReq struct {
	g.Meta          `path:"/account/ssh-public-keys/{keyNo}" method:"delete" tags:"IDC Customer SSH Keys" summary:"Revoke my SSH public key" dc:"Revoke one active owned SSH public key. History is retained and the key can no longer be selected for provisioning or reinstall."`
	KeyNo           string `json:"keyNo" in:"path" v:"required|max-length:40" dc:"Stable owned SSH public-key number" eg:"SSH_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Version last read by the customer" eg:"1"`
	RequestNo       string `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Client-generated retry key reused only for this exact revoke request" eg:"ssh-revoke-550e8400-e29b-41d4-a716-446655440000"`
}

// SSHPublicKeyRevokeRes returns the retained revoked projection.
type SSHPublicKeyRevokeRes struct {
	Item *SSHPublicKeyItem `json:"item" dc:"Revoked SSH public-key projection" eg:"{}"`
}
