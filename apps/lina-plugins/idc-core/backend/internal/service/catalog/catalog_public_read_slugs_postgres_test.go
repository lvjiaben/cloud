// PostgreSQL batch reads intersect exact public slugs with the existing saleable scope before paging.

package catalog

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPublicCatalogSlugSetPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, publicNumberedFixtureSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET summary_zh_cn='真实商品摘要',summary_en_us='Real product summary' WHERE public_slug='alpha-one'`); err != nil {
		t.Fatal(err)
	}
	reader, err := NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	slugs := []string{"alpha-two", "alpha-one", "alpha-draft", "alpha-disabled", "alpha-no-price", "alpha-partial", "alpha-mixed", "absent"}
	page, err := reader.Search(ctx, PublicCatalogSearchInput{PublicSlugs: slugs, Limit: 24})
	if err != nil || len(page.Products) != 2 || page.Products[0].PublicSlug != "alpha-one" || page.Products[1].PublicSlug != "alpha-two" {
		t.Fatalf("saleable set=%+v err=%v", page, err)
	}
	if page.Products[0].NameEnUS != "Alpha one" || page.Products[0].SummaryEnUS != "Real product summary" {
		t.Fatalf("sales card projection lost: %+v", page.Products[0])
	}
	exact, err := reader.Search(ctx, PublicCatalogSearchInput{PublicSlugs: []string{"alpha-two"}, Limit: 24})
	if err != nil || len(exact.Products) != 1 || exact.Products[0].PublicSlug != "alpha-two" {
		t.Fatalf("exact filter=%+v err=%v", exact, err)
	}
	for _, input := range []PublicCatalogSearchInput{
		{PublicSlugs: slugs, Currency: "USD"},
		{PublicSlugs: slugs, NamePrefix: "Beta"},
		{PublicSlugs: slugs, CategoryNo: "ctg_22222222222222222222222222222222"},
		{PublicSlugs: []string{"alpha"}},
	} {
		empty, queryErr := reader.Search(ctx, input)
		if queryErr != nil || len(empty.Products) != 0 {
			t.Fatalf("filters did not intersect: %+v result=%+v err=%v", input, empty, queryErr)
		}
	}
	first, err := reader.Search(ctx, PublicCatalogSearchInput{PublicSlugs: slugs, Limit: 1})
	if err != nil || len(first.Products) != 1 || first.Products[0].PublicSlug != "alpha-one" || first.NextCursor == "" {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	second, err := reader.Search(ctx, PublicCatalogSearchInput{PublicSlugs: slugs, Limit: 1, Cursor: first.NextCursor})
	if err != nil || len(second.Products) != 1 || second.Products[0].PublicSlug != "alpha-two" || second.NextCursor != "" {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
	if _, err = reader.Search(ctx, PublicCatalogSearchInput{PublicSlugs: []string{"alpha-two"}, Limit: 1, Cursor: first.NextCursor}); err == nil {
		t.Fatal("cursor reused for a different set")
	}
	old, err := reader.Search(ctx, PublicCatalogSearchInput{Limit: 24})
	if err != nil || len(old.Products) != 2 {
		t.Fatalf("unfiltered request changed: %+v err=%v", old, err)
	}
}
