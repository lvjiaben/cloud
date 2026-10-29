package v1

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestGenericCatalogCreateRequestsAreBounded(t *testing.T) {
	category := &CatalogCategoryCreateReq{NameZhCn: strings.Repeat("证", 128), NameEnUs: strings.Repeat("a", 128)}
	if err := g.Validator().Data(category).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	category.NameEnUs += "a"
	if err := g.Validator().Data(category).Run(context.Background()); err == nil {
		t.Fatal("overlong category accepted")
	}
	product := &CatalogProductCreateReq{CategoryNo: "ctg_0123456789abcdef0123456789abcdef", PublicSlug: "managed-ssl", NameZhCn: "托管 SSL 证书", NameEnUs: "Managed SSL", SummaryZhCn: strings.Repeat("证", 1024)}
	if err := g.Validator().Data(product).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	product.SummaryZhCn += "证"
	if err := g.Validator().Data(product).Run(context.Background()); err == nil {
		t.Fatal("overlong summary accepted")
	}
}
