// This file defines the neutral typed manifest snapshot primitive shared by
// source-plugin upgrade callbacks and dynamic-plugin lifecycle requests. The
// type is transport-agnostic; bridge wire naming and versioning are owned by
// pluginbridge/contract through type aliases.

package capmodel

import pluginv1 "lina-core/api/plugin/v1"

// ManifestSnapshot is the typed manifest snapshot published to plugin
// lifecycle callbacks.
type ManifestSnapshot struct {
	// ID is the plugin identifier recorded in the manifest snapshot.
	ID string `json:"id"`
	// Name is the plugin display name recorded in the manifest snapshot.
	Name string `json:"name"`
	// Version is the plugin version recorded in the manifest snapshot.
	Version string `json:"version"`
	// Type is the plugin type recorded in the manifest snapshot.
	Type pluginv1.PluginType `json:"type"`
	// Distribution is the plugin distribution governance type recorded in the manifest snapshot.
	Distribution pluginv1.PluginDistribution `json:"distribution"`
	// ScopeNature is the plugin tenant-scope nature recorded in the manifest snapshot.
	ScopeNature pluginv1.ScopeNature `json:"scopeNature"`
	// SupportsMultiTenant reports whether the plugin declares tenant-core support.
	SupportsMultiTenant bool `json:"supportsMultiTenant"`
	// DefaultInstallMode is the plugin default installation mode.
	DefaultInstallMode pluginv1.InstallMode `json:"defaultInstallMode"`
	// Description is the plugin description recorded in the manifest snapshot.
	Description string `json:"description"`
	// BusinessExtensions is the effective owner-authorized business extension set.
	BusinessExtensions []*BusinessExtensionDeclaration `json:"businessExtensions,omitempty"`
	// AdminSurfaces is the governed administration-console surface set.
	AdminSurfaces []*FrontendSurfaceDeclaration `json:"adminSurfaces,omitempty"`
	// CustomerSurfaces is the governed customer-center surface set.
	CustomerSurfaces []*FrontendSurfaceDeclaration `json:"customerSurfaces,omitempty"`
	// InstallSQLCount is the number of install SQL assets recorded in the snapshot.
	InstallSQLCount int `json:"installSqlCount"`
	// UninstallSQLCount is the number of uninstall SQL assets recorded in the snapshot.
	UninstallSQLCount int `json:"uninstallSqlCount"`
	// MockSQLCount is the number of mock SQL assets recorded in the snapshot.
	MockSQLCount int `json:"mockSqlCount"`
	// MenuCount is the number of menu definitions recorded in the snapshot.
	MenuCount int `json:"menuCount"`
	// BackendHookCount is the number of backend hook registrations recorded in the snapshot.
	BackendHookCount int `json:"backendHookCount"`
	// ResourceSpecCount is the number of resource specs recorded in the snapshot.
	ResourceSpecCount int `json:"resourceSpecCount"`
	// HostServiceAuthRequired reports whether host-service authorization is required.
	HostServiceAuthRequired bool `json:"hostServiceAuthRequired"`
}
