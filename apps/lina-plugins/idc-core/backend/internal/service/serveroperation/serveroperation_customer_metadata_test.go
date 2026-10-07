package serveroperation

import (
	"strings"
	"testing"
)

func TestValidCustomerMetadataRejectsUnsafeOrOversizedText(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]string{
		"control":      "hello\nworld",
		"oversized":    strings.Repeat("服", 129),
		"invalid utf8": string([]byte{0xff}),
	} {
		value := value
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if validCustomerMetadata(value, 128) {
				t.Fatalf("unsafe customer metadata %q was accepted", name)
			}
		})
	}
	if !validCustomerMetadata(strings.Repeat("服", 128), 128) {
		t.Fatal("valid Unicode customer metadata was rejected")
	}
}
