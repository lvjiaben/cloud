package staffaccess

import (
	"context"
	"strings"
	"testing"
	"time"

	"lina-plugin-staff-access/backend/cap/staffaccesscap"
)

func TestConstructionAndBindingFailClosed(t *testing.T) {
	if service, err := New(nil, nil, nil, nil, nil); err == nil || service != nil {
		t.Fatalf("New() = (%v, %v), want unavailable dependencies", service, err)
	}
	if client, err := Bind(nil, "idc-core"); err == nil || client != nil {
		t.Fatalf("Bind(nil) = (%v, %v), want error", client, err)
	}
	if client, err := Bind(&Service{}, "  "); err == nil || client != nil {
		t.Fatalf("Bind(blank caller) = (%v, %v), want error", client, err)
	}
}

func TestSaveRejectsInvalidVersionAndValidityBeforeMutation(t *testing.T) {
	service := &Service{}
	base := SaveInput{
		UserID:          1001,
		CapabilityCode:  "idc.support.handle",
		ScopeType:       staffaccesscap.ScopeDepartment,
		ScopeKey:        "DEP_SUPPORT",
		Enabled:         true,
		Reason:          "approved for support",
		ExpectedVersion: -1,
	}
	if _, err := service.Save(context.Background(), base); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("Save(negative version) error = %v, want version rejection", err)
	}

	from := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	until := from
	base.ExpectedVersion = 0
	base.ValidFrom = &from
	base.ValidUntil = &until
	if _, err := service.Save(context.Background(), base); err == nil || !strings.Contains(err.Error(), "expiry") {
		t.Fatalf("Save(invalid validity) error = %v, want expiry rejection", err)
	}
}

func TestBoundClientRejectsUnsafeBatchesBeforeLookup(t *testing.T) {
	client := &boundClient{service: &Service{}, caller: "idc-core"}
	request := staffaccesscap.BatchCheckRequest{
		UserIDs:        []int64{1001, 1001},
		CapabilityCode: "idc.support.handle",
		ScopeType:      staffaccesscap.ScopeDepartment,
		ScopeKey:       "DEP_SUPPORT",
	}
	if _, err := client.BatchCheck(context.Background(), request); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("BatchCheck(duplicate) error = %v, want duplicate rejection", err)
	}

	request.UserIDs = make([]int64, staffaccesscap.MaximumBatchSize+1)
	for index := range request.UserIDs {
		request.UserIDs[index] = int64(index + 1)
	}
	if _, err := client.BatchCheck(context.Background(), request); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("BatchCheck(oversized) error = %v, want bounded rejection", err)
	}
}
