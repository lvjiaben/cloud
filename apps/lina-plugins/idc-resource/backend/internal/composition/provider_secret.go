// Package composition owns source-provider construction seams for the resource plugin.
package composition

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	resourceOwnerPluginID = "idc-resource"
	channelScopePrefix    = "resource.channel.c"
)

// LoadSecretSource freezes the declaration phase and constructs the one
// core-owned source secret capability. It is a startup composition operation,
// not a runtime service locator.
func LoadSecretSource(ctx context.Context) (secretcap.Source, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource secret source construction context is invalid")
	}
	snapshot, err := secretspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze resource secret source declaration")
	}
	factory := snapshot.Factory()
	if factory == nil {
		return nil, gerror.New("resource secret source factory is unavailable")
	}
	source, err := factory(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "construct resource secret source")
	}
	if isNilInterface(source) {
		return nil, gerror.New("resource secret source factory returned nil")
	}
	return source, nil
}

// InjectProviderSecrets derives an unlinkable exact channel scope and replaces
// ProviderEnv.Secrets with the corresponding callback-only accessor. The
// caller must still populate and validate every other ProviderEnv dependency.
func InjectProviderSecrets(
	source secretcap.Source,
	channelRef string,
	env resourcecap.ProviderEnv,
) (resourcecap.ProviderEnv, error) {
	if isNilInterface(source) {
		return resourcecap.ProviderEnv{}, gerror.New("resource secret source is required")
	}
	scope, err := providerSecretScope(channelRef)
	if err != nil {
		return resourcecap.ProviderEnv{}, err
	}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return resourcecap.ProviderEnv{}, gerror.Wrap(err, "bind resource Provider secret accessor")
	}
	if isNilInterface(accessor) {
		return resourcecap.ProviderEnv{}, gerror.New("resource Provider secret accessor is unavailable")
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return resourcecap.ProviderEnv{}, gerror.Wrap(err, "bind resource Provider secret manager")
	}
	if isNilInterface(manager) {
		return resourcecap.ProviderEnv{}, gerror.New("resource Provider secret manager is unavailable")
	}
	env.Secrets = accessor
	env.SecretSink = &channelSecretSink{manager: manager}
	return env, nil
}

// BindProviderSecretManager returns the lifecycle manager for the same exact
// channel scope used by Provider accessors. It is intended for owner-side
// operation credentials during startup-composed service construction.
func BindProviderSecretManager(
	source secretcap.Source,
	channelRef string,
) (secretcap.Manager, error) {
	if isNilInterface(source) {
		return nil, gerror.New("resource secret source is required")
	}
	scope, err := providerSecretScope(channelRef)
	if err != nil {
		return nil, err
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, gerror.Wrap(err, "bind resource Provider secret manager")
	}
	if isNilInterface(manager) {
		return nil, gerror.New("resource Provider secret manager is unavailable")
	}
	return manager, nil
}

// BindProviderSecretAccessor returns the callback-only accessor for the exact
// channel scope used by Provider operation outputs.
func BindProviderSecretAccessor(source secretcap.Source, channelRef string) (secretcap.Accessor, error) {
	if isNilInterface(source) {
		return nil, gerror.New("resource secret source is required")
	}
	scope, err := providerSecretScope(channelRef)
	if err != nil {
		return nil, err
	}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return nil, gerror.Wrap(err, "bind resource Provider secret accessor")
	}
	if isNilInterface(accessor) {
		return nil, gerror.New("resource Provider secret accessor is unavailable")
	}
	return accessor, nil
}

type channelSecretSink struct {
	manager secretcap.Manager
}

var _ resourcecap.SecretSink = (*channelSecretSink)(nil)

func (s *channelSecretSink) Store(
	ctx context.Context,
	request resourcecap.SecretStoreRequest,
	produce resourcecap.SecretProducer,
) (secretcap.Handle, error) {
	if ctx == nil || ctx.Err() != nil || isNilInterface(s) || isNilInterface(s.manager) || produce == nil {
		return secretcap.Handle{}, gerror.New("resource secret output storage failed")
	}
	if err := request.Validate(); err != nil {
		return secretcap.Handle{}, err
	}
	result, err := s.manager.Create(ctx, secretOutputIdempotencyKey(request), func(consume secretcap.Consumer) error {
		return produce(func(plaintext []byte) error {
			return consume(plaintext)
		})
	})
	if err != nil || result.Handle.IsZero() {
		return secretcap.Handle{}, gerror.New("resource secret output storage failed")
	}
	return result.Handle, nil
}

func secretOutputIdempotencyKey(request resourcecap.SecretStoreRequest) string {
	payload := make([]byte, 0, 64+len(request.RequestID)+len(request.Capability)+len(request.Name)+len(request.Policy))
	payload = appendDigestField(payload, request.RequestID)
	payload = appendDigestField(payload, string(request.Capability))
	payload = appendDigestField(payload, request.Name)
	payload = appendDigestField(payload, string(request.Policy))
	var expires [8]byte
	binary.BigEndian.PutUint64(expires[:], uint64(request.ExpiresAt.UTC().UnixMicro()))
	payload = append(payload, expires[:]...)
	digest := sha256.Sum256(payload)
	return "resource-secret-output-" + hex.EncodeToString(digest[:])
}

func appendDigestField(destination []byte, value string) []byte {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	destination = append(destination, size[:]...)
	return append(destination, value...)
}

func providerSecretScope(channelRef string) (secretcap.Scope, error) {
	if err := resourcecap.ValidateChannelRef(channelRef); err != nil {
		return secretcap.Scope{}, gerror.Wrap(err, "derive resource Provider secret scope")
	}
	digest := sha256.Sum256([]byte(channelRef))
	return secretcap.Scope{
		TenantID:      idccap.DefaultTenantID,
		OwnerPluginID: resourceOwnerPluginID,
		CallerScope:   channelScopePrefix + hex.EncodeToString(digest[:16]),
	}, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
