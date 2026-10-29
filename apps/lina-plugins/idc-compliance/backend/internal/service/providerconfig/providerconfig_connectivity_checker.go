// This file binds the fixed Aliyun FACE client to a read-only connectivity checker.
package providerconfig

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const connectivityProbeTimeout = 10 * time.Second

type aliyunConnectivityChecker struct {
	accessor   secretcap.Accessor
	httpClient *http.Client
	clock      func() time.Time
	nonce      func() (string, error)
}

var _ ConnectivityChecker = (*aliyunConnectivityChecker)(nil)

// NewAliyunConnectivityChecker constructs the fixed-endpoint, bounded-timeout
// checker from the callback-only credential accessor.
func NewAliyunConnectivityChecker(accessor secretcap.Accessor) (ConnectivityChecker, error) {
	if nilDependency(accessor) {
		return nil, gerror.New("Aliyun connectivity credential accessor is invalid")
	}
	return &aliyunConnectivityChecker{
		accessor: accessor,
		httpClient: &http.Client{
			Timeout:   connectivityProbeTimeout,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}},
		},
		clock: time.Now,
		nonce: randomProbeNonce,
	}, nil
}

func (c *aliyunConnectivityChecker) Check(
	ctx context.Context,
	handle secretcap.Handle,
	sceneID string,
) ValidationStatus {
	if c == nil {
		return ValidationStatusUnavailable
	}
	client, err := aliyunface.New(
		c.accessor, handle, c.httpClient, c.clock, c.nonce,
	)
	if err != nil {
		return ValidationStatusUnavailable
	}
	switch client.Probe(ctx, sceneID) {
	case aliyunface.ProbeOutcomeReachable:
		return ValidationStatusReachable
	case aliyunface.ProbeOutcomeRejected:
		return ValidationStatusRejected
	default:
		return ValidationStatusUnavailable
	}
}

func randomProbeNonce() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
