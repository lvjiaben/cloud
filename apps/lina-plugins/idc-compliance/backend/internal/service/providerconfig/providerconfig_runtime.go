// This file exposes a private, authorization-free runtime projection to the
// compliance plugin's own personal-verification service. It is not an HTTP or
// cross-plugin capability.
package providerconfig

import (
	"context"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

// RuntimeConfiguration is the exact configured provider generation captured by one attempt.
type RuntimeConfiguration struct {
	ConfigurationNo      string
	SceneID              string
	Version              int64
	CredentialGeneration int64
	Credential           secretcap.Handle
}

// RuntimeSource resolves the enabled configuration for server-side flows only.
type RuntimeSource interface {
	Resolve(context.Context) (RuntimeConfiguration, error)
}

type runtimeSource struct{ manager secretcap.Manager }

var _ RuntimeSource = (*runtimeSource)(nil)

// NewRuntimeSource creates the private server-side configuration projection.
func NewRuntimeSource(manager secretcap.Manager) (RuntimeSource, error) {
	if nilRuntimeDependency(manager) {
		return nil, gerror.New("provider runtime configuration manager is required")
	}
	return &runtimeSource{manager: manager}, nil
}

func (s *runtimeSource) Resolve(ctx context.Context) (RuntimeConfiguration, error) {
	if ctx == nil || ctx.Err() != nil {
		return RuntimeConfiguration{}, gerror.New("provider runtime configuration is unavailable")
	}
	columns := dao.ProviderConfiguration.Columns()
	var row *entity.ProviderConfiguration
	err := dao.ProviderConfiguration.Ctx(ctx).
		Fields(
			columns.TenantId, columns.ConfigurationNo, columns.ProviderCode,
			columns.StatusCode, columns.SceneId, columns.CredentialSecretRef,
			columns.CredentialGeneration, columns.Version,
		).
		Where(do.ProviderConfiguration{TenantId: idccap.DefaultTenantID, ProviderCode: providerCode}).
		Limit(1).Scan(&row)
	if err != nil {
		return RuntimeConfiguration{}, gerror.Wrap(err, "load provider runtime configuration")
	}
	if row == nil || row.ConfigurationNo != configurationNo || row.ProviderCode != providerCode ||
		Status(row.StatusCode) != statusConfigured || !validSceneID(row.SceneId) ||
		row.CredentialSecretRef == nil || row.CredentialGeneration <= 0 || row.Version <= 0 {
		return RuntimeConfiguration{}, gerror.New("provider runtime configuration is unavailable")
	}
	handle, err := secretcap.NewHandle(*row.CredentialSecretRef)
	if err != nil {
		return RuntimeConfiguration{}, gerror.New("provider runtime credential is unavailable")
	}
	metadata, err := s.manager.Metadata(ctx, handle)
	if err != nil || !metadata.Configured || metadata.UpdatedAt.IsZero() {
		return RuntimeConfiguration{}, gerror.New("provider runtime credential is unavailable")
	}
	return RuntimeConfiguration{
		ConfigurationNo: row.ConfigurationNo, SceneID: row.SceneId,
		Version: row.Version, CredentialGeneration: row.CredentialGeneration,
		Credential: handle,
	}, nil
}

func nilRuntimeDependency(value any) bool {
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
