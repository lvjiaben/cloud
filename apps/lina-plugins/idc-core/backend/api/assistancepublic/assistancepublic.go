package assistancepublic

import (
	"context"

	"lina-plugin-idc-core/backend/api/assistancepublic/v1"
)

type IAssistancePublicV1 interface {
	CustomerAssistanceSnapshot(context.Context, *v1.CustomerAssistanceSnapshotReq) (*v1.CustomerAssistanceSnapshotRes, error)
}
