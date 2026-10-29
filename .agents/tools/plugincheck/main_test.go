// This file verifies that audit subprocesses cannot inherit a shared Go workspace.
package main

import "testing"

func TestWithoutWorkPreservesOtherEnvironment(t *testing.T) {
	result := withoutWork([]string{"PATH=tools", "GOWORK=shared", "gowork=other", "CUSTOM=unchanged"})
	if len(result) != 2 || result[0] != "PATH=tools" || result[1] != "CUSTOM=unchanged" {
		t.Fatalf("unexpected subprocess environment: %v", result)
	}
}
