// This file decodes bounded GET configuration selections. It never accepts
// arbitrary nested query binding or silently truncates oversized JSON.
package channel

import (
	"encoding/json"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func configurationQueryValues(raw string) ([]channelcommercecap.PurchaseValue, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 32768 || !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return nil, bizerr.NewCode(channel.CodePurchaseInvalid)
	}
	var values []channelcommercecap.PurchaseValue
	if json.Unmarshal([]byte(raw), &values) != nil || len(values) > 128 {
		return nil, bizerr.NewCode(channel.CodePurchaseInvalid)
	}
	return values, nil
}
