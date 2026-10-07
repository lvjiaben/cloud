package admin

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEncodeNonSecretSettings(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		input   map[string]any
		want    map[string]any
		omitted bool
	}{
		"omitted": {
			omitted: true,
		},
		"empty object": {
			input: map[string]any{},
			want:  map[string]any{},
		},
		"structured values": {
			input: map[string]any{
				"endpoint": "https://dcim.example.test/api",
				"protocol": "legacy",
			},
			want: map[string]any{
				"endpoint": "https://dcim.example.test/api",
				"protocol": "legacy",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			encoded, err := encodeNonSecretSettings(testCase.input)
			if err != nil {
				t.Fatalf("encode non-secret settings: %v", err)
			}
			if testCase.omitted {
				if encoded != nil {
					t.Fatalf("expected omitted settings, got %s", string(encoded))
				}
				return
			}
			var got map[string]any
			if err = json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("decode non-secret settings: %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("unexpected settings: got %#v want %#v", got, testCase.want)
			}
		})
	}
}
