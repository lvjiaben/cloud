// This file blocks new automatic product publication until supplier domain
// capabilities are documented. It performs no outbound calls and leaves frozen
// existing certificate fulfillment and service management unchanged.

package backend

import (
	"bytes"
	"encoding/json"
	"io"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func executeProductConfigurationValidation(payload json.RawMessage) (json.RawMessage, error) {
	var request productfulfillmentcap.ConfigurationValidationRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || request.Version != productfulfillmentcap.ContractVersion || !json.Valid(request.ProductConfiguration) {
		return nil, io.ErrUnexpectedEOF
	}
	// Product names and DNS/file validation flags do not establish supported
	// identifier kinds or maximum SAN count. No current plan has that evidence.
	return json.Marshal(productfulfillmentcap.ConfigurationValidationResult{
		Version: productfulfillmentcap.ContractVersion, Valid: false,
		Reason: productfulfillmentcap.ConfigurationValidationSupplierCapabilitiesUnconfirmed,
	})
}
