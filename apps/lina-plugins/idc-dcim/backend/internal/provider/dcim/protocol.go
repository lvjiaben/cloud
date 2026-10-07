// This file validates the protocol frozen in the exact interface revision.

package dcim

import (
	"bytes"
	"encoding/json"
	"io"
)

// Protocol identifies the exact DCIM API family frozen on one interface revision.
type Protocol string

const (
	// ProtocolLegacy selects the old action-based DCIM API.
	ProtocolLegacy Protocol = "legacy"
	// ProtocolBMS selects the resource-oriented BMS API.
	ProtocolBMS Protocol = "bms"
)

type interfaceSettings struct {
	Protocol Protocol `json:"protocol"`
}

func decodeProtocol(raw json.RawMessage) (Protocol, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var settings interfaceSettings
	if decoder.Decode(&settings) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		(settings.Protocol != ProtocolLegacy && settings.Protocol != ProtocolBMS) {
		return "", errInvalidConfiguration
	}
	return settings.Protocol, nil
}
