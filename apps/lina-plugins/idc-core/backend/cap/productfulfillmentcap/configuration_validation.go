// This file defines optional, pure merchant configuration validation before
// saving or publishing a product. It carries no order identity or credentials.

package productfulfillmentcap

import "encoding/json"

// MethodValidateProductConfiguration is opt-in on exact provider evidence.
// Providers must perform no outbound calls or writes for this method.
const MethodValidateProductConfiguration = "product.configuration.validate"

// ConfigurationValidationReason identifies a bounded, owner-translated rejection.
type ConfigurationValidationReason string

// ConfigurationValidationSupplierCapabilitiesUnconfirmed requires supplier evidence.
const ConfigurationValidationSupplierCapabilitiesUnconfirmed ConfigurationValidationReason = "supplier_capabilities_unconfirmed"

// ConfigurationValidationRequest contains canonical, secret-free merchant values.
type ConfigurationValidationRequest struct {
	Version              string          `json:"version"`
	ProductConfiguration json.RawMessage `json:"productConfiguration"`
}

// ConfigurationValidationResult rejects invalid cross-field combinations;
// failures or malformed results must never be interpreted as approval.
type ConfigurationValidationResult struct {
	Version string                        `json:"version"`
	Valid   bool                          `json:"valid"`
	Reason  ConfigurationValidationReason `json:"reason,omitempty"`
}
