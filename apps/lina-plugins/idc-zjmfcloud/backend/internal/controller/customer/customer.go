// This file defines the customer server controller and its resource-owner dependency.

package customer

import (
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	resourcebridge "lina-plugin-idc-resource/backend/cap/resourcecap/bridge"
	customerapi "lina-plugin-idc-zjmfcloud/backend/api/customer"
)

// Controller projects the resource owner's current-customer contract into this plugin's HTTP DTOs.
type Controller struct {
	servers resourcecap.CurrentCustomerServerService
	initErr error
}

// New constructs the controller for generated WASM route binding with the shared guest host-call transport.
func New() *Controller {
	client, err := resourcebridge.NewGuestClient()
	return &Controller{servers: client, initErr: err}
}

// NewWithService constructs the controller with an explicitly injected startup-owned service client.
func NewWithService(servers resourcecap.CurrentCustomerServerService) *Controller {
	return &Controller{servers: servers}
}

var _ customerapi.ICustomerV1 = (*Controller)(nil)
