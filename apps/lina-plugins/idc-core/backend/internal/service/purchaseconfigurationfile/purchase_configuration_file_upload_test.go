package purchaseconfigurationfile

import (
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestSingleOrderEvidenceResultFailsClosedOnInvalidCardinality(t *testing.T) {
	t.Parallel()
	for name, result := range map[string][][]VerifiedField{
		"empty":    nil,
		"multiple": {{}, {}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			verified, err := singleOrderEvidenceResult(result, nil)
			if verified != nil || !bizerr.Is(err, CodeUnavailable) {
				t.Fatalf("single result = %#v, err = %v", verified, err)
			}
		})
	}
	want := errors.New("query failed")
	if verified, err := singleOrderEvidenceResult([][]VerifiedField{{}}, want); verified != nil || !errors.Is(err, want) {
		t.Fatalf("original error was not preserved: result=%#v err=%v", verified, err)
	}
}
