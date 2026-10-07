// This file appends package versions through the authorized owner boundary.

package admin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// TrafficPackageUpdate makes the route authoritative over body/query identity.
func (c *ControllerV1) TrafficPackageUpdate(ctx context.Context, req *v1.TrafficPackageUpdateReq) (res *v1.TrafficPackageUpdateRes, err error) {
	input := trafficPackageInput(req.TrafficPackageFields)
	input.PackageNo, input.ExpectedVersion = req.PackageNo, req.ExpectedVersion
	if request := g.RequestFromCtx(ctx); request != nil {
		input.PackageNo = request.GetRouter("packageNo").String()
	}
	value, err := c.accessoryPricing.SaveTrafficPackage(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.TrafficPackageUpdateRes{Package: trafficPackageDTO(value)}, nil
}
