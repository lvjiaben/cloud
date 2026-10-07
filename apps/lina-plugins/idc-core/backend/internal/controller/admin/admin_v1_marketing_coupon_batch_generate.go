package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingCouponBatchGenerate(
	ctx context.Context,
	req *v1.MarketingCouponBatchGenerateReq,
) (*v1.MarketingCouponBatchGenerateRes, error) {
	if c.marketingCouponBatches == nil {
		return nil, gerror.New("coupon batch generation is unavailable")
	}
	file, err := c.marketingCouponBatches.Generate(ctx, marketing.CouponBatchGenerateInput{
		RequestNo: req.RequestNo, CampaignNo: req.CampaignNo, BenefitNo: req.BenefitNo,
		CodeLength: req.CodeLength, Count: req.Count,
	})
	if err != nil {
		return nil, err
	}
	request := g.RequestFromCtx(ctx)
	if request == nil || file == nil || len(file.Body) == 0 {
		return nil, gerror.New("coupon batch download is unavailable")
	}
	request.Response.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	request.Response.Header().Set("Content-Length", strconv.Itoa(len(file.Body)))
	safeName := strings.NewReplacer("\\", "_", "\"", "_", "\r", "_", "\n", "_").Replace(file.FileName)
	request.Response.Header().Set("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	if _, err = request.Response.RawWriter().Write(file.Body); err != nil {
		return nil, err
	}
	request.ExitAll()
	return &v1.MarketingCouponBatchGenerateRes{}, nil
}
