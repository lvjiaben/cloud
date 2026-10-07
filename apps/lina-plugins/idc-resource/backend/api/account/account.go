package account

import (
	"context"

	"lina-plugin-idc-resource/backend/api/account/v1"
)

// IAccountV1 exposes only customer-owned server directory reads.
type IAccountV1 interface {
	ServerList(context.Context, *v1.ServerListReq) (*v1.ServerListRes, error)
	ServerDetail(context.Context, *v1.ServerDetailReq) (*v1.ServerDetailRes, error)
}
