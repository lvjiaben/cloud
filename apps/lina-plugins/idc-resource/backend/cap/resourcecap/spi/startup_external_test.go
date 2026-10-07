// This file verifies a source adapter init declaration reaches the resource-owner freeze snapshot.

package spi_test

import (
	"context"
	"strings"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap/spi"
)

// initDeclarationError captures the external-package init declaration result.
var initDeclarationError error

// init simulates a separately compiled source Provider adapter declaration.
func init() {
	initDeclarationError = spi.Declare(spi.Declaration{
		Key: resourcecap.ProviderKey{
			PluginID:        "external-provider-plugin",
			ProviderID:      "external-provider",
			ContractVersion: resourcecap.ContractVersion,
		},
		Factory: externalFactory,
	})
}

// TestInitDeclarationReachesFrozenStartupSnapshot verifies the package seam is
// shared, deterministic, idempotently frozen, and permanently closed.
func TestInitDeclarationReachesFrozenStartupSnapshot(t *testing.T) {
	if initDeclarationError != nil {
		t.Fatalf("external init declaration failed: %v", initDeclarationError)
	}
	snapshot, err := spi.Freeze()
	if err != nil {
		t.Fatalf("freeze startup declarations: %v", err)
	}
	found := false
	for _, declaration := range snapshot.Declarations() {
		if declaration.Key.PluginID == "external-provider-plugin" &&
			declaration.Key.ProviderID == "external-provider" {
			found = true
		}
	}
	if !found {
		t.Fatal("external init declaration is absent from the startup snapshot")
	}
	repeated, err := spi.Freeze()
	if err != nil || repeated.Len() != snapshot.Len() {
		t.Fatalf("repeated freeze changed snapshot: len=%d err=%v", repeated.Len(), err)
	}
	err = spi.Declare(spi.Declaration{
		Key: resourcecap.ProviderKey{
			PluginID:        "late-provider-plugin",
			ProviderID:      "late-provider",
			ContractVersion: resourcecap.ContractVersion,
		},
		Factory: externalFactory,
	})
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("post-freeze declaration error=%v, want frozen rejection", err)
	}
}

// externalFactory is the separately compiled declaration's inert test factory.
func externalFactory(context.Context, resourcecap.ProviderEnv, resourcecap.ProviderConfig) (resourcecap.Provider, error) {
	return nil, nil
}
