// This file creates bounded upstream account passwords only inside synchronous
// encrypted-secret producers. Retries recover the committed handle, not a new value.
package principalprovision

import (
	"crypto/rand"
	"io"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"math/big"
	"strings"
)

const principalPasswordLength = 20

var principalPasswordAlphabets = []string{"abcdefghijklmnopqrstuvwxyz", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "0123456789", "!@#%"}

func principalPasswordProducer(source io.Reader) secretcap.PlaintextProducer {
	return func(consume secretcap.Consumer) error {
		all := strings.Join(principalPasswordAlphabets, "")
		value := make([]byte, principalPasswordLength)
		defer clear(value)
		for i := range value {
			choices := all
			if i < len(principalPasswordAlphabets) {
				choices = principalPasswordAlphabets[i]
			}
			n, err := rand.Int(source, big.NewInt(int64(len(choices))))
			if err != nil {
				return err
			}
			value[i] = choices[n.Int64()]
		}
		for i := len(value) - 1; i > 0; i-- {
			n, err := rand.Int(source, big.NewInt(int64(i+1)))
			if err != nil {
				return err
			}
			j := int(n.Int64())
			value[i], value[j] = value[j], value[i]
		}
		return consume(value)
	}
}
