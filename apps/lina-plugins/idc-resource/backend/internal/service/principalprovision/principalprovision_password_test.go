// This file verifies supplier-compatible account passwords and plaintext lifetime.
package principalprovision

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPrincipalPasswordHasBoundedLengthAndRequiredClasses(t *testing.T) {
	for range 32 {
		var view []byte
		err := principalPasswordProducer(rand.Reader)(func(value []byte) error {
			view = value
			if len(value) != 20 {
				t.Fatal("upstream password exceeds allowed length")
			}
			for _, alphabet := range principalPasswordAlphabets {
				if !strings.ContainsAny(string(value), alphabet) {
					t.Fatal("required password character class missing")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range view {
			if v != 0 {
				t.Fatal("password retained after encryption callback")
			}
		}
	}
}
func TestPrincipalPasswordEntropyFailureDoesNotCallConsumer(t *testing.T) {
	called := false
	err := principalPasswordProducer(io.LimitReader(strings.NewReader(""), 0))(func([]byte) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("entropy failure generated a credential")
	}
	marker := errors.New("consumer failed")
	if err = principalPasswordProducer(rand.Reader)(func([]byte) error { return marker }); !errors.Is(err, marker) {
		t.Fatal("consumer failure lost")
	}
}
