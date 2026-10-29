// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerLoginContact is the golang structure for table customer_login_contact.
type CustomerLoginContact struct {
	TenantId           int       `json:"tenantId"           orm:"tenant_id"           description:""`
	UserId             int       `json:"userId"             orm:"user_id"             description:""`
	EmailDigest        string    `json:"emailDigest"        orm:"email_digest"        description:""`
	PhoneDigest        string    `json:"phoneDigest"        orm:"phone_digest"        description:""`
	IdentityCiphertext string    `json:"identityCiphertext" orm:"identity_ciphertext" description:""`
	Revision           int64     `json:"revision"           orm:"revision"            description:""`
	CreatedAt          time.Time `json:"createdAt"          orm:"created_at"          description:""`
	UpdatedAt          time.Time `json:"updatedAt"          orm:"updated_at"          description:""`
}
