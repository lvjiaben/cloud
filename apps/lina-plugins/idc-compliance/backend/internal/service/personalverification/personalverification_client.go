// This file binds immutable provider configurations to scoped Aliyun clients.

package personalverification

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const providerCallTimeout = 15 * time.Second

type aliyunClientFactory struct {
	accessor secretcap.Accessor
	client   *http.Client
	clock    func() time.Time
}

var _ ClientFactory = (*aliyunClientFactory)(nil)

// NewAliyunClientFactory constructs a shared bounded HTTP client factory.
func NewAliyunClientFactory(accessor secretcap.Accessor) (ClientFactory, error) {
	if nilDependency(accessor) {
		return nil, gerror.New("personal verification credential accessor is invalid")
	}
	return &aliyunClientFactory{
		accessor: accessor, client: &http.Client{Timeout: providerCallTimeout}, clock: time.Now,
	}, nil
}

func (f *aliyunClientFactory) Create(
	configuration providerconfig.RuntimeConfiguration,
) (aliyunface.Client, error) {
	if f == nil || configuration.Credential.IsZero() || configuration.Version <= 0 ||
		configuration.SceneID == "" {
		return nil, gerror.New("personal verification provider configuration is invalid")
	}
	return aliyunface.New(f.accessor, configuration.Credential, f.client, f.clock, providerNonce)
}

func providerNonce() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
