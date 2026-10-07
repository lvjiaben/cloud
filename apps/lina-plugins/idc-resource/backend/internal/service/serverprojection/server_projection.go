// Package serverprojection converts the private resource-owned server directory
// into the customer-safe owner host-service contract.
package serverprojection

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverdirectory"
)

type directory interface {
	List(context.Context, serverdirectory.ListRequest) (*serverdirectory.Page, error)
	Detail(context.Context, serverdirectory.DetailRequest) (*serverdirectory.Item, error)
}

// Service implements the resource owner's current-customer server reads.
type Service struct {
	customers customerauthcap.Reader
	directory directory
}

var _ resourcecap.CurrentCustomerServerService = (*Service)(nil)

// New constructs one server projection service from explicitly injected
// current-customer and resource-directory dependencies.
func New(customers customerauthcap.Reader, directory directory) (*Service, error) {
	if customers == nil || directory == nil {
		return nil, gerror.New("resource current customer server projection dependencies are required")
	}
	return &Service{customers: customers, directory: directory}, nil
}
