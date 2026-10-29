// This file freezes the typed configuration through the existing owner seam.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerTargetQuote(ctx context.Context, req *v1.NativeServerTargetQuoteReq) (*v1.NativeServerTargetQuoteRes, error) {
	if err := bindNativePath(ctx, "serverNo", &req.ServerNo); err != nil {
		return nil, publicError(err)
	}
	input, err := configurationValues(req.Values)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := c.commercial.QuoteTarget(ctx, customeroperations.ConfigurationCommand{ServerNo: req.ServerNo, ConfigurationRevision: req.ConfigurationRevision, BillingTermToken: req.BillingTermToken, Values: input})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	if item == nil {
		return nil, publicError(customeroperations.ErrUnavailable)
	}
	return &v1.NativeServerTargetQuoteRes{QuoteNo: item.QuoteNo, QuoteItemNo: item.QuoteItemNo, StructuredConfigurationNo: item.StructuredConfigurationNo}, nil
}
