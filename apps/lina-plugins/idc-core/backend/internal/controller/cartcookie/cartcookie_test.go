package cartcookie

import (
	"net/http"
	"testing"
)

func TestNewProtectsGuestCartCredential(t *testing.T) {
	cookie := New("gcart_secret", true)
	if cookie.Name != Name || cookie.Value != "gcart_secret" || cookie.Path != "/" {
		t.Fatalf("unexpected cookie identity: %+v", cookie)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("guest credential cookie is not protected: %+v", cookie)
	}
	if cookie.MaxAge <= 0 || cookie.Expires.IsZero() {
		t.Fatalf("guest credential cookie must have a bounded lifetime: %+v", cookie)
	}
}

func TestNewAllowsLocalHTTPWithoutWeakeningProductionPolicy(t *testing.T) {
	cookie := New("gcart_local", false)
	if cookie.Secure {
		t.Fatal("local HTTP cookie must remain usable for fixed-port development")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("local cookie lost browser protections: %+v", cookie)
	}
}
