package customerprojection

import (
	"context"
	"reflect"
	"testing"
)

type scannerStub struct{ calls int }

func (s *scannerStub) GetScan(_ context.Context, destination any, _ string, _ ...any) error {
	s.calls++
	rows := destination.(*[]row)
	*rows = []row{{CustomerNo: "CUS_hidden", CustomerCode: 10008, DisplayName: "Example", PhoneCountryCode: "+86", Phone: "13800138000"}}
	return nil
}

func TestLoadDeduplicatesAndReturnsDisplaySafeSummary(t *testing.T) {
	reader := &scannerStub{}
	result, err := Load(context.Background(), reader, []string{"CUS_hidden", "CUS_hidden"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("query calls = %d, want 1", reader.calls)
	}
	want := Summary{CustomerCode: "10008", DisplayName: "Example", PrimaryPhone: "+8613800138000"}
	if !reflect.DeepEqual(result["CUS_hidden"], want) {
		t.Fatalf("summary = %#v, want %#v", result["CUS_hidden"], want)
	}
}
