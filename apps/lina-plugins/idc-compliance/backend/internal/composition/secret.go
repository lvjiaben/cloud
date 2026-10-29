// Package composition owns explicit startup seams for the compliance source plugin.
package composition

import (
	"context"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
)

const (
	complianceOwnerPluginID = "idc-compliance"
	aliyunCredentialScope   = "compliance.aliyun-face-credential"
	localSupplierScope      = "compliance.local-supplier-credential"
	personalSnapshotScope   = "compliance.personal-identity-snapshot"
	enterpriseMaterialScope = "compliance.enterprise-material-encryption"
	taxInvoiceSnapshotScope = "compliance.tax-invoice-snapshot"
)

// BindTaxInvoiceSnapshotSecrets isolates immutable buyer, seller, allocation,
// and tax-item snapshots from provider credentials.
func BindTaxInvoiceSnapshotSecrets(
	source secretcap.Source,
) (secretcap.Accessor, secretcap.Manager, error) {
	if nilInterface(source) {
		return nil, nil, gerror.New("compliance secret source is required")
	}
	scope := secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: complianceOwnerPluginID,
		CallerScope: taxInvoiceSnapshotScope,
	}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind tax invoice snapshot accessor")
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind tax invoice snapshot manager")
	}
	if nilInterface(accessor) || nilInterface(manager) {
		return nil, nil, gerror.New("tax invoice snapshot secret boundary is unavailable")
	}
	return accessor, manager, nil
}

// LoadSecretSource freezes declaration and constructs the core-owned source
// secret capability during plugin startup composition.
func LoadSecretSource(ctx context.Context) (secretcap.Source, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("compliance secret source construction context is invalid")
	}
	snapshot, err := secretspi.Freeze()
	if err != nil {
		return nil, gerror.Wrap(err, "freeze compliance secret source declaration")
	}
	factory := snapshot.Factory()
	if factory == nil {
		return nil, gerror.New("compliance secret source factory is unavailable")
	}
	source, err := factory(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "construct compliance secret source")
	}
	if nilInterface(source) {
		return nil, gerror.New("compliance secret source factory returned nil")
	}
	return source, nil
}

// BindEnterpriseMaterialSecrets returns the isolated callback-only boundary
// for enterprise legal snapshots and document envelopes. The generated key is
// installation-owned and its opaque handle never leaves compliance persistence.
func BindEnterpriseMaterialSecrets(
	source secretcap.Source,
) (secretcap.Accessor, secretcap.Manager, error) {
	if nilInterface(source) {
		return nil, nil, gerror.New("compliance secret source is required")
	}
	scope := secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: complianceOwnerPluginID,
		CallerScope: enterpriseMaterialScope,
	}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind enterprise material accessor")
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind enterprise material manager")
	}
	if nilInterface(accessor) || nilInterface(manager) {
		return nil, nil, gerror.New("enterprise material secret boundary is unavailable")
	}
	if _, ok := manager.(secretcap.GeneratedManager); !ok {
		return nil, nil, gerror.New("enterprise material generated-secret boundary is unavailable")
	}
	return accessor, manager, nil
}

// BindPersonalIdentitySecrets returns the exact callback-only boundary for
// encrypted legal-profile snapshots. It is separated from provider credentials
// so one purpose can never access the other scope.
func BindPersonalIdentitySecrets(
	source secretcap.Source,
) (secretcap.Accessor, secretcap.Manager, error) {
	if nilInterface(source) {
		return nil, nil, gerror.New("compliance secret source is required")
	}
	scope := secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: complianceOwnerPluginID,
		CallerScope: personalSnapshotScope,
	}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind personal identity snapshot accessor")
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind personal identity snapshot manager")
	}
	if nilInterface(accessor) || nilInterface(manager) {
		return nil, nil, gerror.New("personal identity snapshot secret boundary is unavailable")
	}
	return accessor, manager, nil
}

// BindAliyunCredentialSecrets returns the exact callback-only accessor and
// lifecycle manager for the one installation-wide Aliyun FACE configuration.
func BindAliyunCredentialSecrets(
	source secretcap.Source,
) (secretcap.Accessor, secretcap.Manager, error) {
	if nilInterface(source) {
		return nil, nil, gerror.New("compliance secret source is required")
	}
	scope := secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: complianceOwnerPluginID,
		CallerScope: aliyunCredentialScope,
	}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind Aliyun credential accessor")
	}
	if nilInterface(accessor) {
		return nil, nil, gerror.New("Aliyun credential accessor is unavailable")
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, nil, gerror.Wrap(err, "bind Aliyun credential manager")
	}
	if nilInterface(manager) {
		return nil, nil, gerror.New("Aliyun credential manager is unavailable")
	}
	return accessor, manager, nil
}

// BindLocalSupplierCredentialSecrets returns only the lifecycle manager for
// merchant-owned Nuonuo credential envelopes. No plaintext
// accessor is composed before an independently confirmed real client exists.
func BindLocalSupplierCredentialSecrets(source secretcap.Source) (secretcap.Manager, error) {
	if nilInterface(source) {
		return nil, gerror.New("compliance secret source is required")
	}
	scope := secretcap.Scope{
		TenantID: idccap.DefaultTenantID, OwnerPluginID: complianceOwnerPluginID,
		CallerScope: localSupplierScope,
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, gerror.Wrap(err, "bind local supplier credential manager")
	}
	if nilInterface(manager) {
		return nil, gerror.New("local supplier credential manager is unavailable")
	}
	return manager, nil
}

func nilInterface(value any) bool {
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
