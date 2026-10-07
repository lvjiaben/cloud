// This file provides the process startup-only Provider declaration and freeze seam.

package spi

import "lina-plugin-idc-resource/backend/cap/resourcecap"

var (
	// startupRegistrar is the single declaration sink shared by source-adapter
	// init functions and the resource owner's later startup composition. It is not
	// a runtime Provider directory and is intentionally never reset.
	startupRegistrar, startupRegistrarErr = newRegistrar(resourcecap.ContractVersion)
)

// Declare records one source Provider factory during process initialization.
// Callers normally invoke it from their plugin init function and must treat an
// error as startup-fatal. Declarations after the first Freeze are rejected.
func Declare(declaration Declaration) error {
	if startupRegistrarErr != nil {
		return startupRegistrarErr
	}
	return startupRegistrar.declare(declaration)
}

// Freeze permanently closes process declarations and returns a deterministic
// detached snapshot for resource-owner startup composition. Repeated calls are
// idempotent; no runtime lookup or Provider invocation is exposed.
func Freeze() (Snapshot, error) {
	if startupRegistrarErr != nil {
		return Snapshot{}, startupRegistrarErr
	}
	return startupRegistrar.freeze()
}
