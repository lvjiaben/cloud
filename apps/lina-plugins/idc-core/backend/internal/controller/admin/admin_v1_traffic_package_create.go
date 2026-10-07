// This file delegates traffic-package creation to its authorized business owner.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// TrafficPackageCreate never derives a price or product scope in the controller.
func (c *ControllerV1) TrafficPackageCreate(ctx context.Context, req *v1.TrafficPackageCreateReq) (res *v1.TrafficPackageCreateRes, err error) {
	value, err := c.accessoryPricing.SaveTrafficPackage(ctx, trafficPackageInput(req.TrafficPackageFields))
	if err != nil {
		return nil, err
	}
	return &v1.TrafficPackageCreateRes{Package: trafficPackageDTO(value)}, nil
}
