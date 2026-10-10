// Package adminserver adapts fixed-provider staff directory reads to HTTP.
package adminserver

import (
	"context"
	"lina-core/pkg/bizerr"
	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
	"lina-plugin-idc-resource/backend/api/adminserver/v1"
	"lina-plugin-idc-resource/backend/internal/service/serverdirectory"
)

type Reader interface {
	List(context.Context, int, int) (*serverdirectory.Page, error)
	Detail(context.Context, string) (*serverdirectory.Item, error)
}
type Controller struct{ directory Reader }

func New(directory Reader) (*Controller, error) {
	if directory == nil {
		return nil, bizerr.NewCode(serverdirectory.CodeDirectoryAdminUnavailable)
	}
	return &Controller{directory: directory}, nil
}
func (c *Controller) CloudServerList(ctx context.Context, req *v1.CloudServerListReq) (*v1.CloudServerListRes, error) {
	result, err := c.directory.List(ctx, req.Page, req.Limit)
	if err != nil {
		return nil, err
	}
	rows := make([]*accountv1.ServerItem, len(result.Items))
	for i := range result.Items {
		rows[i] = project(result.Items[i])
	}
	return &v1.CloudServerListRes{Items: rows, Total: result.Total, NextCursor: result.NextCursor}, nil
}
func (c *Controller) CloudServerDetail(ctx context.Context, req *v1.CloudServerDetailReq) (*v1.CloudServerDetailRes, error) {
	row, err := c.directory.Detail(ctx, req.ServerNo)
	if err != nil {
		return nil, err
	}
	return &v1.CloudServerDetailRes{Item: project(*row)}, nil
}
func project(row serverdirectory.Item) *accountv1.ServerItem {
	return &accountv1.ServerItem{ServerNo: row.ServerNo, ProductNameZhCN: row.ProductNameZhCN, ProductNameEnUS: row.ProductNameEnUS, ChannelName: row.ChannelName, PoolName: row.PoolName, Kind: string(row.Kind), State: string(row.State), LatestOperation: string(row.LatestOperation), AvailableActions: []string{}, BoundAt: row.BoundAt.UnixMilli(), UpdatedAt: row.UpdatedAt.UnixMilli()}
}
