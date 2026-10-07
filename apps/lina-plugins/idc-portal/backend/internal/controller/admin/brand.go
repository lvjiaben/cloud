package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-portal/backend/api/admin/v1"
)

func (c *ControllerV1) BrandGet(_ context.Context, _ *v1.BrandGetReq) (*v1.BrandGetRes, error) {
	config, published, err := c.brands.Current()
	if err != nil {
		return nil, err
	}
	res := &v1.BrandGetRes{Published: published}
	if published {
		res.Brand = &config
	}
	return res, nil
}

func (c *ControllerV1) BrandSave(_ context.Context, req *v1.BrandSaveReq) (*v1.BrandSaveRes, error) {
	if req == nil {
		return nil, gerror.New("brand request is required")
	}
	config, err := c.brands.Save(req.Brand)
	if err != nil {
		return nil, err
	}
	return &v1.BrandSaveRes{Brand: config}, nil
}

func (c *ControllerV1) BrandPreview(_ context.Context, req *v1.BrandPreviewReq) (*v1.BrandPreviewRes, error) {
	if req == nil {
		return nil, gerror.New("brand revision is required")
	}
	config, err := c.brands.Preview(req.Revision)
	if err != nil {
		return nil, err
	}
	return &v1.BrandPreviewRes{Brand: config}, nil
}

func (c *ControllerV1) BrandPublish(_ context.Context, req *v1.BrandPublishReq) (*v1.BrandPublishRes, error) {
	if req == nil {
		return nil, gerror.New("brand revision is required")
	}
	if err := c.brands.Publish(req.Revision); err != nil {
		return nil, err
	}
	return &v1.BrandPublishRes{Published: true}, nil
}

func (c *ControllerV1) BrandRollback(_ context.Context, _ *v1.BrandRollbackReq) (*v1.BrandRollbackRes, error) {
	if err := c.brands.Rollback(); err != nil {
		return nil, err
	}
	return &v1.BrandRollbackRes{RolledBack: true}, nil
}
