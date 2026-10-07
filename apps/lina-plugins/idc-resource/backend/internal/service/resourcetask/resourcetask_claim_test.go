package resourcetask

import (
	"testing"
	"time"
)

func TestClaimInputFailsClosedBeforeDatabaseAccess(t *testing.T) {
	for _, input := range []ClaimInput{
		{WorkerID: "", Limit: 1, LeaseDuration: time.Minute},
		{WorkerID: "worker/invalid", Limit: 1, LeaseDuration: time.Minute},
		{WorkerID: "worker-1", Limit: 0, LeaseDuration: time.Minute},
		{WorkerID: "worker-1", Limit: maximumClaimBatchSize + 1, LeaseDuration: time.Minute},
		{WorkerID: "worker-1", Limit: 1, LeaseDuration: time.Millisecond},
		{WorkerID: "worker-1", Limit: 1, LeaseDuration: maximumLeaseDuration + time.Second},
	} {
		if err := validateClaimInput(input); err == nil {
			t.Fatalf("invalid claim input was accepted: %+v", input)
		}
	}
	if err := validateClaimInput(ClaimInput{
		WorkerID: "worker-1", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil {
		t.Fatalf("valid claim input was rejected: %v", err)
	}
}

func TestLeaseTokenIsBoundedAndWorkerScoped(t *testing.T) {
	token, err := newLeaseToken(zeroReader{}, "worker-1")
	if err != nil || token != "worker-1:00000000000000000000000000000000" {
		t.Fatalf("unexpected deterministic lease token %q err=%v", token, err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(target []byte) (int, error) {
	clear(target)
	return len(target), nil
}
