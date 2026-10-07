// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RateWindow is the golang structure for table rate_window.
type RateWindow struct {
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	DimensionCode   string     `json:"dimensionCode"   orm:"dimension_code"    description:""`
	SubjectDigest   string     `json:"subjectDigest"   orm:"subject_digest"    description:""`
	WindowSeconds   int        `json:"windowSeconds"   orm:"window_seconds"    description:""`
	WindowStartedAt *time.Time `json:"windowStartedAt" orm:"window_started_at" description:""`
	Counter         int        `json:"counter"         orm:"counter"           description:""`
	ExpiresAt       *time.Time `json:"expiresAt"       orm:"expires_at"        description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
