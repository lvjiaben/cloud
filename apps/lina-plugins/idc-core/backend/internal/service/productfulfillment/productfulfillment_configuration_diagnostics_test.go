// This file ensures frozen-configuration diagnostics cannot echo provider secrets.
package productfulfillment

import (
	"errors"
	"testing"
)

func TestFrozenConfigurationDiagnosticRedactsUnknownErrors(t *testing.T) {
	if got := frozenConfigurationFailureReason(errors.New("private-token must never enter logs")); got != "unclassified" {
		t.Fatal(got)
	}
	if got := frozenConfigurationFailureReason(errors.New("frozen configuration provider evidence is unavailable")); got != "provider_missing" {
		t.Fatal(got)
	}
	if got := frozenConfigurationFailureReason(nil); got != "evidence_count_mismatch" {
		t.Fatal(got)
	}
}
