// This file defines the versioned customer controller interface used by the WASM route generator.

package customer

import (
	"context"

	v1 "lina-plugin-idc-zjmfcloud/backend/api/customer/v1"
)

// ICustomerV1 lists the customer server endpoints implemented by this plugin.
type ICustomerV1 interface {
	ServerList(context.Context, *v1.ServerListReq) (*v1.ServerListRes, error)
	ServerDetail(context.Context, *v1.ServerDetailReq) (*v1.ServerDetailRes, error)
}
