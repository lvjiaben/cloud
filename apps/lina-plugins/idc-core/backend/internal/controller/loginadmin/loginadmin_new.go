// This file injects the customer-owned login directory into its admin route.
package loginadmin

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
)

type ControllerV1 struct{ directory customerlogincap.Directory }

func NewV1(directory customerlogincap.Directory) (*ControllerV1, error) {
	if directory == nil {
		return nil, gerror.New("customer login directory is required")
	}
	return &ControllerV1{directory: directory}, nil
}
