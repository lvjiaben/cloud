// Package peer implements the customer-bound same-system public protocol.
// Credentials remain in the resource owner's confidential outbound boundary.
package peer

import (
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"net/http"
	"time"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var (
	errInvalid     = gerror.New("same-system request is invalid")
	errUnavailable = gerror.New("same-system response is unavailable")
	errRejected    = gerror.New("same-system request was rejected")
)

const apiPath = "/x/idc-channel/api/v1/channel"

// client is one invocation, never a shared session. Each outbound call consumes
// a monotonically increasing sequence under the owner-issued context.
type client struct {
	executor   confidentialoutboundcap.Service
	contextID  string
	sequence   uint8
	observedAt time.Time
}

func (c *client) call(method, path string, query map[string][]string, body any, fields []string, target any) error {
	if c.executor == nil || c.contextID == "" || c.sequence >= 15 {
		return errInvalid
	}
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: method, Path: apiPath + path, Query: query, UseAuthentication: true, ResponseFields: append([]string{"code", "data"}, fields...)}
	if body != nil {
		var err error
		request.JSONBody, err = json.Marshal(body)
		if err != nil {
			return errInvalid
		}
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return errInvalid
	}
	c.sequence++
	raw, err := c.executor.Execute(c.contextID, c.sequence, payload)
	if err != nil {
		return errUnavailable
	}
	var response resourcecap.ConfidentialHTTPResponse
	if json.Unmarshal(raw, &response) != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return errUnavailable
	}
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(response.Data, &envelope) != nil || envelope.Code == nil {
		return errUnavailable
	}
	if *envelope.Code != 0 {
		return errRejected
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errUnavailable
	}
	if target != nil && json.Unmarshal(envelope.Data, target) != nil {
		return errUnavailable
	}
	return nil
}

func (c *client) verify() error {
	var result struct {
		GenericAPI bool  `json:"genericApi"`
		ObservedAt int64 `json:"observedAt"`
	}
	if err := c.call(http.MethodGet, "/capabilities", nil, nil, []string{"genericApi", "observedAt"}, &result); err != nil {
		return err
	}
	if !result.GenericAPI || !validObservation(result.ObservedAt) {
		return errUnavailable
	}
	c.observedAt = time.UnixMilli(result.ObservedAt).UTC()
	return nil
}

func validObservation(value int64) bool {
	return value > 0 && value < 253402300800000
}
