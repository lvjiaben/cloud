// Slug-set tests guard anonymous batch bounds and cursor binding without changing unfiltered requests.

package catalog

import (
	"reflect"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestPublicCatalogSlugSetValidationAndCursorBinding(t *testing.T) {
	codec := testPublicCatalogCursorCodec(t)
	input := PublicCatalogSearchInput{PublicSlugs: []string{"beta", "alpha", "alpha"}, Limit: 1}
	normalized, err := normalizePublicCatalogSearch(input, codec)
	if err != nil || !reflect.DeepEqual(normalized.publicSlugs, []string{"alpha", "beta"}) {
		t.Fatalf("normalized=%+v err=%v", normalized, err)
	}
	if !reflect.DeepEqual(input.PublicSlugs, []string{"beta", "alpha", "alpha"}) {
		t.Fatal("normalization mutated caller input")
	}
	cursor, err := codec.encode(publicCatalogCursorBoundary{ID: 1, SortOrder: 1}, normalized)
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := normalizePublicCatalogSearch(PublicCatalogSearchInput{PublicSlugs: []string{"alpha", "beta"}, Limit: 1, Cursor: cursor}, codec)
	if err != nil || reordered.cursor == nil {
		t.Fatalf("same set rejected: %v", err)
	}
	for _, changed := range []PublicCatalogSearchInput{
		{PublicSlugs: []string{"alpha"}, Limit: 1, Cursor: cursor},
		{Limit: 1, Cursor: cursor},
		{PublicSlugs: []string{"alpha", "beta"}, Limit: 2, Cursor: cursor},
		{PublicSlugs: []string{"alpha", "beta"}, Limit: 1, NamePrefix: "other", Cursor: cursor},
	} {
		if _, err = normalizePublicCatalogSearch(changed, codec); !bizerr.Is(err, CodePublicCatalogInvalid) {
			t.Fatalf("changed filter accepted: %+v err=%v", changed, err)
		}
	}
	overLimit := make([]string, 25)
	for i := range overLimit {
		overLimit[i] = "alpha"
	}
	for _, slugs := range [][]string{{""}, {"bad/slug"}, {"a--b"}, {"Alpha"}, {strings.Repeat("a", 129)}, overLimit} {
		if _, err = normalizePublicCatalogSearch(PublicCatalogSearchInput{PublicSlugs: slugs}, codec); !bizerr.Is(err, CodePublicCatalogInvalid) {
			t.Fatalf("invalid set accepted: %v err=%v", slugs, err)
		}
	}
	bounded := make([]string, 24)
	for i := range bounded {
		bounded[i] = "alpha"
	}
	if _, err = normalizePublicCatalogSearch(PublicCatalogSearchInput{PublicSlugs: bounded}, codec); err != nil {
		t.Fatalf("24 entries rejected: %v", err)
	}
	unfiltered, err := normalizePublicCatalogSearch(PublicCatalogSearchInput{}, codec)
	if err != nil || len(unfiltered.publicSlugs) != 0 || unfiltered.limit != 12 {
		t.Fatalf("unfiltered behavior changed: %+v %v", unfiltered, err)
	}
}
