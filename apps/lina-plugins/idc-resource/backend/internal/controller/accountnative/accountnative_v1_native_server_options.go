// This file maps complete resource-owner directories into customer choices.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerOptions(ctx context.Context, req *v1.NativeServerOptionsReq) (*v1.NativeServerOptionsRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	options, err := c.service.Options(ctx, req.ServerNo)
	if err != nil {
		return nil, publicError(err)
	}
	result := &v1.NativeServerOptionsRes{Images: []v1.NativeServerImage{}, Accessories: []v1.NativeServerAccessory{}}
	if options.Images != nil {
		stamp := options.Images.ObservedAt.UnixMilli()
		result.ImagesObservedAt = &stamp
		for _, item := range options.Images.Images {
			result.Images = append(result.Images, v1.NativeServerImage{Reference: item.Reference, Name: item.Name, Group: item.Group, OSType: item.OSType, Arch: item.Arch, BootMode: item.BootMode, Selectable: item.Selectable})
		}
	}
	if options.Accessories != nil {
		stamp := options.Accessories.ObservedAt.UnixMilli()
		result.AccessoriesObservedAt = &stamp
		expiry := options.Accessories.ExpiresAt.UnixMilli()
		result.AccessoriesExpiresAt = &expiry
		for _, item := range options.Accessories.Items {
			result.Accessories = append(result.Accessories, v1.NativeServerAccessory{ObjectNo: item.ObjectNo, Kind: string(item.Kind), Name: item.Name, State: string(item.State), Projection: item.Projection})
		}
	}
	return result, nil
}
