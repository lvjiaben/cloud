// This file rejects undeclared Provider task takeover JSON fields without
// moving authorization or business validation out of the owner service.

package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"

	"lina-plugin-idc-resource/backend/api/admin/v1"
)

type providerTaskTakeoverBody struct {
	ExpectedVersion int64    `json:"expectedVersion"`
	ReasonCode      string   `json:"reasonCode"`
	EvidenceNos     []string `json:"evidenceNos"`
}

const maximumProviderTaskTakeoverBodySize = 8 * 1024

// providerTaskTakeoverBodyViolation detects unknown, trailing, or
// binder-inconsistent JSON. The service still authorizes before rejecting it.
func providerTaskTakeoverBodyViolation(raw []byte, req *v1.ProviderTaskManualTakeoverReq) bool {
	if req == nil || len(raw) > maximumProviderTaskTakeoverBodySize || len(bytes.TrimSpace(raw)) == 0 {
		return true
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var body providerTaskTakeoverBody
	if err := decoder.Decode(&body); err != nil {
		return true
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return true
	}
	return body.ExpectedVersion != req.ExpectedVersion ||
		body.ReasonCode != req.ReasonCode ||
		!slices.Equal(body.EvidenceNos, req.EvidenceNos)
}
