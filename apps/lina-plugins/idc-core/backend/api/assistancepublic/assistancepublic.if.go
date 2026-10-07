// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package assistancepublic

import (
	"context"

	"lina-plugin-idc-core/backend/api/assistancepublic/v1"
)

type IAssistancepublicV1 interface {
	CustomerAssistanceSnapshot(ctx context.Context, req *v1.CustomerAssistanceSnapshotReq) (res *v1.CustomerAssistanceSnapshotRes, err error)
}
