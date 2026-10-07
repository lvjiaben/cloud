// This file verifies opaque resource mapping number generation.

package resourcetask

import "testing"

// TestNewMappingNumber verifies the stable unpredictable mapping number shape.
func TestNewMappingNumber(t *testing.T) {
	t.Parallel()
	number, err := newMappingNumber(zeroReader{})
	if err != nil || number != "mapping_00000000000000000000000000000000" {
		t.Fatalf("unexpected mapping number: number=%q err=%v", number, err)
	}
}
