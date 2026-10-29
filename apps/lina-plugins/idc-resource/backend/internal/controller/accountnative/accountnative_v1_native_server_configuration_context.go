// This file binds the actual route and projects a core-owned dynamic form.
package accountnative

import (
	"context"
	"encoding/json"
	"io"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
	"strings"
)

func (c *ControllerV1) NativeServerConfigurationContext(ctx context.Context, req *v1.NativeServerConfigurationContextReq) (*v1.NativeServerConfigurationContextRes, error) {
	if err := bindNativePath(ctx, "serverNo", &req.ServerNo); err != nil {
		return nil, publicError(err)
	}
	var values []*v1.NativeConfigurationValue
	if req.Values != "" {
		decoder := json.NewDecoder(strings.NewReader(req.Values))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&values); err != nil || values == nil || len(values) > 100 {
			return nil, publicError(customeroperations.ErrInvalid)
		}
		var trailing any
		if decoder.Decode(&trailing) != io.EOF {
			return nil, publicError(customeroperations.ErrInvalid)
		}
	}
	input, err := configurationValues(values)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := c.commercial.Configuration(ctx, customeroperations.ConfigurationCommand{ServerNo: req.ServerNo, ConfigurationRevision: req.ConfigurationRevision, Values: input})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	if item == nil {
		return nil, publicError(customeroperations.ErrUnavailable)
	}
	return projectConfiguration(item), nil
}
