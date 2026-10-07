// Package spi defines the declaration-time Provider factory registrar.
package spi

import (
	"sort"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// Declaration associates one complete Provider key with its construction factory.
type Declaration struct {
	// Key is the exact plugin, Provider, and contract-version identity.
	Key resourcecap.ProviderKey
	// Factory constructs the Provider during explicit resource-owner composition.
	Factory resourcecap.ProviderFactory
	// ProvisionParameters maps the server owner's secret-free plan to this
	// Provider's non-secret operation parameters. Providers that do not support
	// server provisioning leave it nil.
	ProvisionParameters resourcecap.ProvisionParameterBuilder
}

// declarationRegistrar accepts Provider declarations until its first freeze call. The
// process package seam owns the only production instance.
type declarationRegistrar struct {
	// mu linearizes declarations and the one-way freeze transition.
	mu sync.Mutex

	// supportedVersion is the owner's exact currently supported contract.
	supportedVersion string
	// frozen marks the irreversible end of the declaration phase.
	frozen bool
	// declarations holds unique validated startup declarations before freeze.
	declarations map[resourcecap.ProviderKey]Declaration
	// frozenSnapshot is the deterministic immutable source copied to callers.
	frozenSnapshot []Declaration
}

// Snapshot is an immutable, deterministic declaration view produced by Freeze.
// It deliberately exposes no runtime Provider lookup or invocation method.
type Snapshot struct {
	// declarations is detached on construction and every public read.
	declarations []Declaration
}

// newRegistrar creates an isolated registrar for startup construction and tests.
func newRegistrar(supportedContractVersion string) (*declarationRegistrar, error) {
	if _, err := resourcecap.ParseContractVersion(supportedContractVersion); err != nil {
		return nil, gerror.Wrap(err, "create resource Provider registrar")
	}
	if supportedContractVersion != resourcecap.ContractVersion {
		return nil, gerror.New("create resource Provider registrar: contract version is outside the owner-supported range")
	}
	return &declarationRegistrar{
		supportedVersion: supportedContractVersion,
		declarations:     make(map[resourcecap.ProviderKey]Declaration),
	}, nil
}

// Declare validates and records one Provider factory. Duplicate exact keys,
// versions outside the owner's explicit support range, nil factories, and
// declarations after Freeze are rejected without changing the registrar.
func (r *declarationRegistrar) declare(declaration Declaration) error {
	if r == nil {
		return gerror.New("resource Provider registrar is nil")
	}
	if err := declaration.Key.Validate(); err != nil {
		return gerror.Wrap(err, "declare resource Provider factory")
	}
	if declaration.Factory == nil {
		return gerror.Newf("resource Provider factory is required: %s", formatKey(declaration.Key))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return gerror.Newf("resource Provider registrar is frozen: %s", formatKey(declaration.Key))
	}
	if declaration.Key.ContractVersion != r.supportedVersion {
		return gerror.New("resource Provider contract version is outside the owner-supported range")
	}
	if _, exists := r.declarations[declaration.Key]; exists {
		return gerror.Newf("resource Provider factory is already declared: %s", formatKey(declaration.Key))
	}
	r.declarations[declaration.Key] = declaration
	return nil
}

// Freeze closes the declaration phase and returns a deterministic immutable
// snapshot. Repeated calls return detached views of the same frozen contents.
func (r *declarationRegistrar) freeze() (Snapshot, error) {
	if r == nil {
		return Snapshot{}, gerror.New("resource Provider registrar is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.frozen {
		r.frozenSnapshot = make([]Declaration, 0, len(r.declarations))
		for _, declaration := range r.declarations {
			r.frozenSnapshot = append(r.frozenSnapshot, declaration)
		}
		sort.Slice(r.frozenSnapshot, func(left int, right int) bool {
			return declarationSortKey(r.frozenSnapshot[left]) < declarationSortKey(r.frozenSnapshot[right])
		})
		r.frozen = true
	}
	return Snapshot{declarations: cloneDeclarations(r.frozenSnapshot)}, nil
}

// Declarations returns a detached copy in deterministic key order.
func (s Snapshot) Declarations() []Declaration {
	return cloneDeclarations(s.declarations)
}

// Len reports the number of declarations in the frozen snapshot.
func (s Snapshot) Len() int {
	return len(s.declarations)
}

// cloneDeclarations returns a detached declaration slice.
func cloneDeclarations(source []Declaration) []Declaration {
	return append([]Declaration(nil), source...)
}

// declarationSortKey creates a deterministic composite ordering key.
func declarationSortKey(declaration Declaration) string {
	return declaration.Key.PluginID + "\x00" +
		declaration.Key.ProviderID + "\x00" +
		declaration.Key.ContractVersion
}

// formatKey formats only already-validated identity fields for startup
// diagnostics.
func formatKey(key resourcecap.ProviderKey) string {
	return "plugin=" + key.PluginID + " provider=" + key.ProviderID + " contract=" + key.ContractVersion
}
