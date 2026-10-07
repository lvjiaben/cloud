// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractArchiveJob is the golang structure for table contract_archive_job.
type ContractArchiveJob struct {
	Id                 int64      `json:"id"                 orm:"id"                   description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	JobNo              string     `json:"jobNo"              orm:"job_no"               description:""`
	ContractNo         string     `json:"contractNo"         orm:"contract_no"          description:""`
	FlowNo             string     `json:"flowNo"             orm:"flow_no"              description:""`
	GatewayRouteNo     string     `json:"gatewayRouteNo"     orm:"gateway_route_no"     description:""`
	ArtifactTypeCode   string     `json:"artifactTypeCode"   orm:"artifact_type_code"   description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	AttemptCount       int        `json:"attemptCount"       orm:"attempt_count"        description:""`
	NextAttemptAt      *time.Time `json:"nextAttemptAt"      orm:"next_attempt_at"      description:""`
	LeaseSha256        string     `json:"leaseSha256"        orm:"lease_sha256"         description:""`
	LeaseExpiresAt     *time.Time `json:"leaseExpiresAt"     orm:"lease_expires_at"     description:""`
	LastReasonCode     string     `json:"lastReasonCode"     orm:"last_reason_code"     description:""`
	SupplierObservedAt *time.Time `json:"supplierObservedAt" orm:"supplier_observed_at" description:""`
	CompletedAt        *time.Time `json:"completedAt"        orm:"completed_at"         description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
}
