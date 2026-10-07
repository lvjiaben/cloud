package numberedpage

import "testing"

func TestPageBounds(t *testing.T) {
	for _, size := range []int{10, 20, 50, 100} {
		if !Valid(1, size, "") || !Valid(1000000, size, "") {
			t.Fatalf("valid size %d rejected", size)
		}
	}
	for _, v := range []struct {
		page, size int
		cursor     string
	}{{-1, 10, ""}, {1000001, 10, ""}, {1, 0, ""}, {1, 101, ""}, {1, 10, "opaque"}} {
		if Valid(v.page, v.size, v.cursor) {
			t.Fatalf("invalid page accepted: %+v", v)
		}
	}
	if !Valid(0, 20, "opaque") {
		t.Fatal("existing cursor consumer rejected")
	}
}
