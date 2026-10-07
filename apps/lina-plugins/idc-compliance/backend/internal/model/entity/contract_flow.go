// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractFlow is the golang structure for table contract_flow.
type ContractFlow struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	FlowNo                string     `json:"flowNo"                orm:"flow_no"                 description:""`
	ContractNo            string     `json:"contractNo"            orm:"contract_no"             description:""`
	SequenceNo            int        `json:"sequenceNo"            orm:"sequence_no"             description:""`
	PurposeCode           string     `json:"purposeCode"           orm:"purpose_code"            description:""`
	ProviderCode          string     `json:"providerCode"          orm:"provider_code"           description:""`
	ExternalFlowRef       *string    `json:"externalFlowRef"       orm:"external_flow_ref"       description:""`
	IdempotencySha256     string     `json:"idempotencySha256"     orm:"idempotency_sha256"      description:""`
	RequestSnapshotRef    string     `json:"requestSnapshotRef"    orm:"request_snapshot_ref"    description:""`
	RequestSnapshotSha256 string     `json:"requestSnapshotSha256" orm:"request_snapshot_sha256" description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	AvailableAt           *time.Time `json:"availableAt"           orm:"available_at"            description:""`
	NextQueryAt           *time.Time `json:"nextQueryAt"           orm:"next_query_at"           description:""`
	ExpiresAt             *time.Time `json:"expiresAt"             orm:"expires_at"              description:""`
	SubmittedAt           *time.Time `json:"submittedAt"           orm:"submitted_at"            description:""`
	CompletedAt           *time.Time `json:"completedAt"           orm:"completed_at"            description:""`
	FailureCode           string     `json:"failureCode"           orm:"failure_code"            description:""`
	Version               int64      `json:"version"               orm:"version"                 description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"              description:""`
	GatewayRouteNo        *string    `json:"gatewayRouteNo"        orm:"gateway_route_no"        description:"Opaque installation-local supplier-write route; never a Tencent flow identifier"`
	ProviderStatus        *string    `json:"providerStatus"        orm:"provider_status"         description:""`
	GatewayRouteVersion   int64      `json:"gatewayRouteVersion"   orm:"gateway_route_version"   description:""`
}
