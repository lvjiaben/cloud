// This file creates shared navigation after trusted service-level authorization.
package directoryadmin

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryadmin/v1"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
)

func (c *ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (*v1.CreateRes, error) {
	directory, err := c.service.Save(ctx, productdirectory.SaveInput{PagePluginIDs: req.PagePluginIDs, NavigationType: productdirectory.NavigationType(req.NavigationType), TargetURL: req.TargetURL, Icon: req.Icon, ParentDirectoryNo: req.ParentDirectoryNo, NameZhCn: req.NameZhCn, NameEnUs: req.NameEnUs, SortOrder: req.SortOrder})
	if err != nil {
		return nil, err
	}
	item := projectDirectory(*directory)
	return &v1.CreateRes{Directory: &item}, nil
}
