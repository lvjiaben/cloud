package externalmenu

import (
	"strings"
	"testing"
)

func TestNormalizePreservesSafeExternalURL(t *testing.T) {
	t.Parallel()

	input := SaveInput{
		NameZh:    "  帮助中心  ",
		NameEn:    "  Help center  ",
		Icon:      "  lucide:circle-help  ",
		URL:       "  https://docs.example.com/help?source=console#start  ",
		SortOrder: 20,
	}
	got, err := normalize(input)
	if err != nil {
		t.Fatalf("normalize safe menu: %v", err)
	}
	if got.NameZh != "帮助中心" || got.NameEn != "Help center" || got.Icon != "lucide:circle-help" {
		t.Fatalf("normalize labels and icon: %#v", got)
	}
	if got.URL != "https://docs.example.com/help?source=console#start" {
		t.Fatalf("external URL changed unexpectedly: %q", got.URL)
	}
}

func TestNormalizeRejectsUnsafeOrInvalidExternalMenu(t *testing.T) {
	t.Parallel()

	valid := SaveInput{
		NameZh:    "帮助中心",
		NameEn:    "Help center",
		Icon:      "lucide:circle-help",
		URL:       "https://docs.example.com/help",
		SortOrder: 20,
	}
	tests := map[string]SaveInput{
		"missing Chinese name": func() SaveInput { value := valid; value.NameZh = ""; return value }(),
		"missing English name": func() SaveInput { value := valid; value.NameEn = ""; return value }(),
		"unsupported icon":     func() SaveInput { value := valid; value.Icon = "custom:unsafe"; return value }(),
		"relative URL":         func() SaveInput { value := valid; value.URL = "/help"; return value }(),
		"script URL":           func() SaveInput { value := valid; value.URL = "javascript:alert(1)"; return value }(),
		"credential URL":       func() SaveInput { value := valid; value.URL = "https://user:secret@example.com"; return value }(),
		"negative sort":        func() SaveInput { value := valid; value.SortOrder = -1; return value }(),
		"oversized sort":       func() SaveInput { value := valid; value.SortOrder = 10_000; return value }(),
		"oversized URL": func() SaveInput {
			value := valid
			value.URL = "https://example.com/" + strings.Repeat("a", 2048)
			return value
		}(),
	}
	for name, input := range tests {
		input := input
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := normalize(input); err == nil {
				t.Fatal("expected invalid external menu to be rejected")
			}
		})
	}
}
