// This file canonicalizes one-time Tencent authorization projections and
// exposes active subjects only through synchronous, clear-after-use callbacks.

package tencentess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"
)

const maximumSubjectBytes = 1024

var errSubjectUnavailable = errors.New("tencent eSign subject is unavailable")

type subjectSource interface {
	WithActiveSubject(context.Context, string, func([]byte) error) error
}

type canonicalSubject struct {
	ProxyOrganizationOpenID string `json:"proxyOrganizationOpenId"`
	ProxyOperatorOpenID     string `json:"proxyOperatorOpenId"`
}

// SubjectCanonicalizer accepts only the server-side one-time authorization
// projection. It is never a browser-facing raw subject input contract.
type SubjectCanonicalizer struct{}

// Canonicalize implements the installation-local supplier identity contract.
func (SubjectCanonicalizer) Canonicalize(ctx context.Context, input []byte) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || len(input) == 0 || len(input) > maximumSubjectBytes {
		return nil, errSubjectUnavailable
	}
	subject, err := decodeSubject(input)
	if err != nil {
		return nil, err
	}
	defer subject.clear()
	result, err := json.Marshal(subject)
	if err != nil || len(result) == 0 || len(result) > maximumSubjectBytes {
		clearBytes(result)
		return nil, errSubjectUnavailable
	}
	return result, nil
}

func (c *Client) withAgent(ctx context.Context, call func(*essbasic.Agent) error) error {
	if !c.Configured() || ctx == nil || ctx.Err() != nil || call == nil {
		return errSubjectUnavailable
	}
	return c.subjects.WithActiveSubject(ctx, providerCode, func(payload []byte) error {
		subject, err := decodeSubject(payload)
		if err != nil {
			return err
		}
		defer subject.clear()
		appID := c.appID
		agent := &essbasic.Agent{
			AppId:                   &appID,
			ProxyOrganizationOpenId: &subject.ProxyOrganizationOpenID,
			ProxyOperator:           &essbasic.UserInfo{OpenId: &subject.ProxyOperatorOpenID},
		}
		return call(agent)
	})
}

func decodeSubject(input []byte) (canonicalSubject, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	var subject canonicalSubject
	if err := decoder.Decode(&subject); err != nil {
		return canonicalSubject{}, errSubjectUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		subject.clear()
		return canonicalSubject{}, errSubjectUnavailable
	}
	if !validOpaque(subject.ProxyOrganizationOpenID, 1, 64) ||
		!validOpaque(subject.ProxyOperatorOpenID, 1, 64) {
		subject.clear()
		return canonicalSubject{}, errSubjectUnavailable
	}
	return subject, nil
}

func (s *canonicalSubject) clear() {
	if s == nil {
		return
	}
	s.ProxyOrganizationOpenID = ""
	s.ProxyOperatorOpenID = ""
}
