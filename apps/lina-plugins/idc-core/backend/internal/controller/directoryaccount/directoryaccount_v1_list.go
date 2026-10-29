// This file projects merchant navigation menus and scoped service counts.
package directoryaccount

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryaccount/v1"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
)

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	page, err := c.service.ListAccount(ctx, req.Page, req.PageSize, productdirectory.NavigationType(req.NavigationType))
	if err != nil {
		return nil, err
	}
	items := make([]v1.Directory, len(page.Items))
	for i, item := range page.Items {
		items[i] = v1.Directory{NavigationType: string(item.NavigationType), TargetURL: item.TargetURL, Icon: item.Icon, ParentDirectoryNo: item.ParentDirectoryNo, DirectoryNo: item.DirectoryNo, NameZhCn: item.NameZhCn, NameEnUs: item.NameEnUs, SortOrder: item.SortOrder, ServiceCount: item.ServiceCount}
	}
	return &v1.ListRes{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}
