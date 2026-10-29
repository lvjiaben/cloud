// This file reuses the shared pre-dispatch wait while retaining resource-owned
// callback observation. Ambiguous or already executed calls remain unrepeatable.

package providerexecution

import (
	"context"
	"lina-plugin-idc-core/backend/cap/provideradmissioncap"
)

// IsAdmissionBusy recognizes only the authoritative host lock refusal.
func IsAdmissionBusy(err error) bool { return provideradmissioncap.IsBusy(err) }

// waitForAdmission preserves the resource adapter's existing callback fence.
func waitForAdmission(ctx context.Context, attempt func() (bool, error)) error {
	return provideradmissioncap.Wait(ctx, attempt)
}
