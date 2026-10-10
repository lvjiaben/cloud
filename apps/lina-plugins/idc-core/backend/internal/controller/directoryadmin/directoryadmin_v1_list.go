// This file projects merchant directories without per-product detail requests.
package directoryadmin

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryadmin/v1"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
)

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	page, err := c.service.ListAdmin(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]v1.Directory, len(page.Items))
	for i, item := range page.Items {
		items[i] = projectDirectory(item)
	}
	return &v1.ListRes{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, nil
}
func projectDirectory(item productdirectory.Directory) v1.Directory {
	products := make([]v1.Product, len(item.Products))
	for i, p := range item.Products {
		products[i] = v1.Product{ProductNo: p.ProductNo, NameZhCn: p.NameZhCn, NameEnUs: p.NameEnUs}
	}
	return v1.Directory{DirectoryNo: item.DirectoryNo, NameZhCn: item.NameZhCn, NameEnUs: item.NameEnUs, SortOrder: item.SortOrder, StatusCode: string(item.StatusCode), Version: item.Version, ProductNos: item.ProductNos, Products: products, ProductCount: item.ProductCount}
}
