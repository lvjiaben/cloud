// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContactVerification is the golang structure for table contact_verification.
type ContactVerification struct {
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	VerificationNo          string     `json:"verificationNo"          orm:"verification_no"           description:""`
	PurposeCode             string     `json:"purposeCode"             orm:"purpose_code"              description:""`
	OperationCode           string     `json:"operationCode"           orm:"operation_code"            description:""`
	DestinationKind         string     `json:"destinationKind"         orm:"destination_kind"          description:""`
	DestinationDigest       string     `json:"destinationDigest"       orm:"destination_digest"        description:""`
	BrowserDigest           string     `json:"browserDigest"           orm:"browser_digest"            description:""`
	TicketDigest            string     `json:"ticketDigest"            orm:"ticket_digest"             description:""`
	LocaleCode              string     `json:"localeCode"              orm:"locale_code"               description:""`
	TemplateNo              string     `json:"templateNo"              orm:"template_no"               description:""`
	TemplateVersion         int64      `json:"templateVersion"         orm:"template_version"          description:""`
	RequestShapeDigest      string     `json:"requestShapeDigest"      orm:"request_shape_digest"      description:""`
	IdempotencyDigest       string     `json:"idempotencyDigest"       orm:"idempotency_digest"        description:""`
	CodeDigest              string     `json:"codeDigest"              orm:"code_digest"               description:""`
	MaskedDestination       string     `json:"maskedDestination"       orm:"masked_destination"        description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	DeliveryState           string     `json:"deliveryState"           orm:"delivery_state"            description:""`
	FailedAttempts          int        `json:"failedAttempts"          orm:"failed_attempts"           description:""`
	RetryAfterAt            *time.Time `json:"retryAfterAt"            orm:"retry_after_at"            description:""`
	ExpiresAt               *time.Time `json:"expiresAt"               orm:"expires_at"                description:""`
	ProviderBoundaryCrossed bool       `json:"providerBoundaryCrossed" orm:"provider_boundary_crossed" description:""`
	VerifiedAt              *time.Time `json:"verifiedAt"              orm:"verified_at"               description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
