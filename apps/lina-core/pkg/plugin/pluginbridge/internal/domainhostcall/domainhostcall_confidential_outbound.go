// This file implements the isolated guest client for host-managed supplier calls.

package domainhostcall

import (
	"encoding/json"

	"lina-core/pkg/plugin/pluginbridge/protocol"
)

// confidentialOutboundService adapts the one-use confidential outbound host service.
type confidentialOutboundService struct{ baseService }

// ConfidentialOutbound creates the confidential outbound guest client.
func ConfidentialOutbound(invoker HostServiceInvoker) *confidentialOutboundService {
	return &confidentialOutboundService{baseService: newBaseServiceWithHostService(nil, invoker)}
}

// Execute consumes one host-bound context and returns only its approved JSON projection.
func (s *confidentialOutboundService) Execute(contextID string, sequence uint8, payload json.RawMessage) (json.RawMessage, error) {
	var response struct {
		Payload json.RawMessage `json:"payload"`
	}
	err := s.callHostServiceJSONRequest(
		protocol.HostServiceConfidentialOutbound,
		protocol.HostServiceMethodConfidentialOutboundExecute,
		"",
		"",
		struct {
			ContextID string          `json:"contextId"`
			Sequence  uint8           `json:"sequence"`
			Payload   json.RawMessage `json:"payload"`
		}{ContextID: contextID, Sequence: sequence, Payload: payload},
		&response,
	)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), response.Payload...), nil
}
