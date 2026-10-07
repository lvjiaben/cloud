package public

import (
	"context"

	"lina-plugin-idc-portal/backend/api/public/v1"
)

func (c *ControllerV1) BrandGet(_ context.Context, _ *v1.BrandGetReq) (*v1.BrandGetRes, error) {
	config, published, err := c.brands.Current()
	if err != nil {
		return nil, err
	}
	if !published {
		return &v1.BrandGetRes{}, nil
	}
	return &v1.BrandGetRes{Brand: &config}, nil
}
