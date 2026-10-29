// These tests exercise real cart transactions with private login records while
// isolating credential creation, which is covered by its owning service tests.
package commerce

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"lina-plugin-idc-core/backend/internal/service/purchaseserverlogin"
)

type cartLoginFixture struct {
	purchaseserverlogin.Service
	database gdb.DB
}

func (f cartLoginFixture) Resolve(ctx context.Context, token, customer, version string) (*purchaseserverlogin.Resolved, error) {
	var row *purchaseserverlogin.Resolved
	err := f.database.GetScan(ctx, &row, `SELECT hostname,password_secret_ref,channel_no,cart_item_no FROM plugin_idc_core_purchase_server_login WHERE server_login_token=? AND customer_no=? AND product_version_no=?`, token, customer, version)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return row, nil
}
func (f cartLoginFixture) Bind(ctx context.Context, token, customer, version, item string) error {
	result, err := f.database.Exec(ctx, `UPDATE plugin_idc_core_purchase_server_login SET cart_item_no=? WHERE server_login_token=? AND customer_no=? AND product_version_no=? AND (cart_item_no='' OR cart_item_no=?)`, item, token, customer, version, item)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func TestServerLoginCartAndFreezeEditsPersistAcrossRequotePostgreSQL(t *testing.T) {
	f := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
	f.service.serverLogins = cartLoginFixture{database: f.database}
	freezer := f.resolver.(catalog.PurchaseConfigurationFreezer)
	consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(f.database, f.service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(f.service, freezer, consumer); err != nil {
		t.Fatal(err)
	}

	token := "PSL_" + strings.Repeat("a", 64)
	if _, err := f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_purchase_server_login(server_login_token,customer_no,user_id,product_version_no,configuration_revision,hostname,password_secret_ref,channel_no)VALUES(?,?,?,?,?,?,?,?)`, token, "customer-commerce-service-1", 101, "version-commerce-service-1", f.purchase.ConfigurationRevision, "first.example", "private-secret-reference", "CHN_cart"); err != nil {
		t.Fatal(err)
	}
	cart, err := f.service.GetAccountCart(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	purchase := f.purchase
	purchase.ServerLoginToken = token
	input := AddItemInput{ExpectedCartVersion: cart.Version, Quantity: 2, Purchase: purchase}
	if _, err = f.service.AddAccountItem(f.ctx, input); err == nil {
		t.Fatal("one typed login accepted multiple servers")
	}
	input.Quantity = 1
	added, err := f.service.AddAccountItem(f.ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var selected CartItem
	for _, item := range added.Items {
		if item.HasServerLogin {
			selected = item
		}
	}
	if selected.CartItemNo == "" || selected.ServerLoginHostname != "first.example" || !selected.ServerLoginPasswordConfigured || selected.ServerLoginToken != token {
		t.Fatal("safe cart login metadata missing")
	}
	repeated, err := f.service.AddAccountItem(f.ctx, input)
	if err != nil || len(repeated.Items) != len(added.Items) {
		t.Fatal("retry duplicated an independent server item")
	}
	if _, err = f.service.UpdateAccountItem(f.ctx, UpdateItemInput{CartItemNo: selected.CartItemNo, ExpectedCartVersion: added.Version, ExpectedItemVersion: selected.Version, Quantity: 2}); err == nil {
		t.Fatal("quantity update copied a private login")
	}
	replacement := f.purchase
	updated, err := f.service.UpdateAccountItem(f.ctx, UpdateItemInput{CartItemNo: selected.CartItemNo, ExpectedCartVersion: added.Version, ExpectedItemVersion: selected.Version, Quantity: 1, Purchase: &replacement})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range updated.Items {
		if item.CartItemNo == selected.CartItemNo && (!item.HasServerLogin || item.ServerLoginToken != token) {
			t.Fatal("configuration edit lost private password")
		}
	}
	quoted, err := f.service.QuoteAccountCart(f.ctx, QuoteRequest{ExpectedCartVersion: updated.Version})
	if err != nil {
		t.Fatal(err)
	}
	editedToken := "PSL_" + strings.Repeat("b", 64)
	if _, err = f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_purchase_server_login(server_login_token,customer_no,user_id,product_version_no,configuration_revision,cart_item_no,hostname,password_secret_ref,channel_no)VALUES(?,?,?,?,?,?,?,?,?)`, editedToken, "customer-commerce-service-1", 101, "version-commerce-service-1", f.purchase.ConfigurationRevision, selected.CartItemNo, "edited.example", "replacement-private-secret-reference", "CHN_cart"); err != nil {
		t.Fatal(err)
	}
	for _, quoteItem := range quoted.Items {
		explicitToken := ""
		if quoteItem.CartItemNo == selected.CartItemNo {
			explicitToken = editedToken
		}
		frozen, freezeErr := f.service.FreezeAccountQuoteItemConfiguration(f.ctx, FreezeQuoteItemConfigurationInput{ServerLoginToken: explicitToken, QuoteItemNo: quoteItem.QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: f.purchase.ConfigurationRevision, Values: f.purchase.Values}})
		if freezeErr != nil {
			t.Fatal(freezeErr)
		}
		if quoteItem.CartItemNo == selected.CartItemNo {
			value, getErr := f.database.GetValue(f.ctx, `SELECT server_login_token FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE snapshot_no=?`, frozen.SnapshotNo)
			if getErr != nil || value.String() != editedToken {
				t.Fatal("quote freeze did not save edited private login")
			}
			repeated, repeatErr := f.service.FreezeAccountQuoteItemConfiguration(f.ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: quoteItem.QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: f.purchase.ConfigurationRevision, Values: f.purchase.Values}})
			if repeatErr != nil || repeated.SnapshotNo != frozen.SnapshotNo {
				t.Fatal("freeze retry did not inherit the newly edited login")
			}
		}
	}
	safeCart, err := f.service.GetAccountCart(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range safeCart.Items {
		if item.CartItemNo == selected.CartItemNo && (item.ServerLoginToken != editedToken || item.ServerLoginHostname != "edited.example" || !item.ServerLoginPasswordConfigured) {
			t.Fatal("freeze edit did not persist current cart login preference")
		}
	}
	quoted, err = f.service.QuoteAccountCart(f.ctx, QuoteRequest{ExpectedCartVersion: safeCart.Version})
	if err != nil {
		t.Fatal(err)
	}
	for _, quoteItem := range quoted.Items {
		frozen, freezeErr := f.service.FreezeAccountQuoteItemConfiguration(f.ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: quoteItem.QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: f.purchase.ConfigurationRevision, Values: f.purchase.Values}})
		if freezeErr != nil {
			t.Fatal(freezeErr)
		}
		if quoteItem.CartItemNo == selected.CartItemNo {
			value, getErr := f.database.GetValue(f.ctx, `SELECT server_login_token FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE snapshot_no=?`, frozen.SnapshotNo)
			if getErr != nil || value.String() != editedToken {
				t.Fatal("requote lost newly edited server login")
			}
		}
	}
	checkout, err := f.service.CheckoutAccountQuote(f.ctx, CheckoutRequest{QuoteNo: quoted.QuoteNo, IdempotencyKey: "customer-login-checkout-1"})
	if err != nil {
		t.Fatal(err)
	}
	count, err := f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE order_no=? AND server_login_token=? AND order_item_no IS NOT NULL`, checkout.OrderNo, editedToken)
	if err != nil || count.Int() != 1 {
		t.Fatal("checkout did not bind exactly one private login")
	}

}
