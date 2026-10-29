// This file replaces a bounded directory configuration at an optimistic version.
package directoryadmin

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryadmin/v1"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
)

func (c *ControllerV1) Update(ctx context.Context, req *v1.UpdateReq) (*v1.UpdateRes, error) {
	directory, err := c.service.Save(ctx, productdirectory.SaveInput{PagePluginIDs: req.PagePluginIDs, NavigationType: productdirectory.NavigationType(req.NavigationType), TargetURL: req.TargetURL, Icon: req.Icon, ParentDirectoryNo: req.ParentDirectoryNo, DirectoryNo: req.DirectoryNo, NameZhCn: req.NameZhCn, NameEnUs: req.NameEnUs, SortOrder: req.SortOrder, ExpectedVersion: req.ExpectedVersion, StatusCode: productdirectory.Status(req.StatusCode)})
	if err != nil {
		return nil, err
	}
	item := projectDirectory(*directory)
	return &v1.UpdateRes{Directory: &item}, nil
}
