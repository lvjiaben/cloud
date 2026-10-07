package secret

import (
	"bytes"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestConsumeProducedPlaintextCopiesAndClearsInput(t *testing.T) {
	input := []byte("source-manager-secret")
	plaintext, err := consumeProducedPlaintext(func(consume secretcap.Consumer) error {
		return consume(input)
	})
	if err != nil || !bytes.Equal(plaintext, []byte("source-manager-secret")) {
		t.Fatalf("consume produced plaintext: value=%q err=%v", plaintext, err)
	}
	if !allZero(input) {
		t.Fatalf("producer input was not cleared: %x", input)
	}
	zeroBytes(plaintext)
}

func TestConsumeProducedPlaintextRejectsBrokenProducer(t *testing.T) {
	tests := []struct {
		name    string
		produce secretcap.PlaintextProducer
	}{
		{name: "nil", produce: nil},
		{name: "missing callback", produce: func(secretcap.Consumer) error { return nil }},
		{name: "empty", produce: func(consume secretcap.Consumer) error { return consume(nil) }},
		{name: "panic", produce: func(secretcap.Consumer) error { panic("sensitive panic") }},
		{name: "double callback", produce: func(consume secretcap.Consumer) error {
			first := []byte("first-secret")
			second := []byte("second-secret")
			_ = consume(first)
			return consume(second)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plaintext, err := consumeProducedPlaintext(test.produce)
			if err == nil || len(plaintext) != 0 {
				t.Fatalf("broken producer accepted: value=%q err=%v", plaintext, err)
			}
			if test.name != "nil" && !bizerr.Is(err, CodeConsumerFailed) {
				t.Fatalf("producer failure was not collapsed: %v", err)
			}
		})
	}
}
