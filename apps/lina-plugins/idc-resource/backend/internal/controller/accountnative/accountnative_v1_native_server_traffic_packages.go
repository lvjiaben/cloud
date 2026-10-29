package accountnative

import (
	"context"
	"strconv"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerTrafficPackages(ctx context.Context, req *v1.NativeServerTrafficPackagesReq) (res *v1.NativeServerTrafficPackagesRes, err error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	items, err := c.commercial.TrafficPackages(ctx, req.ServerNo)
	if err != nil {
		return nil, publicCommercialError(err)
	}
	res = &v1.NativeServerTrafficPackagesRes{Items: make([]v1.NativeServerTrafficPackage, 0, len(items))}
	for _, item := range items {
		res.Items = append(res.Items, v1.NativeServerTrafficPackage{PackageNo: item.PackageNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, CurrencyCode: item.CurrencyCode, TrafficGB: item.TrafficGB, AmountMinor: strconv.FormatInt(item.AmountMinor, 10), RemainingStock: item.RemainingStock, UnlimitedStock: item.UnlimitedStock})
	}
	return res, nil
}
