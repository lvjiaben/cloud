// This file validates an explicitly admitted native remote-evaluation view.
// It contains public configuration and current explanatory terms, never a
// complete static price table, an executable module, or a supplier quote token.
package resourcecap

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// ValidateNativeRemoteSnapshot bounds the common-module view before an owner
// freezes its identity and remote-evaluation mode. Account prices are reevaluated
// for selected values; this method does not confer quote or order authority.
var nativeRemoteRevision = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidateNativeRemoteSnapshot(snapshot *channelcommercecap.ProductSnapshot) error {
	if snapshot == nil || snapshot.Validate() != nil || !nativePositiveID(snapshot.Product.PublicSlug) || !snapshot.PricingReady || len(snapshot.Currencies) != 1 || snapshot.Currencies[0].Precision != 2 || len(snapshot.BillingTerms) != 1 || len(snapshot.Configuration.ConfigurationRevision) != 64 {
		return gerror.New("native remote snapshot is invalid")
	}
	if !nativeRemoteRevision.MatchString(snapshot.Configuration.ConfigurationRevision) {
		return gerror.New("native remote revision is invalid")
	}
	for _, field := range snapshot.Configuration.Fields {
		if !nativeRemoteFieldValid(field) {
			return gerror.New("native remote field mapping is invalid")
		}
	}
	terms := snapshot.BillingTerms[0]
	if terms.CurrencyCode != snapshot.Currencies[0].CurrencyCode || terms.Precision != 2 {
		return gerror.New("native remote currency is invalid")
	}
	for _, term := range terms.Terms {
		token := term.BillingTermToken
		if token != "free" && token != "onetime" && !nativePositiveID(token) || term.MinimumPurchaseCycles != 1 || term.SetupAmount != "0" || term.TaxAmount != "0" || term.SubtotalAmount != term.TotalAmount || term.Status != "available" {
			return gerror.New("native remote billing term is invalid")
		}
	}
	return nil
}
func nativeRemoteFieldValid(field channelcommercecap.ConfigurationField) bool {
	parts := strings.Split(field.FieldToken, ":")
	if len(parts) < 2 || len(parts) > 3 || !nativePositiveID(parts[1]) || !field.Visible || field.Disabled || field.UploadPolicy != nil {
		return false
	}
	switch parts[0] {
	case "config":
		if len(parts) == 3 {
			index, err := strconv.Atoi(parts[2])
			if err != nil || index < 0 || index >= 32 || strconv.Itoa(index) != parts[2] || field.Type != capmodel.ConfigurationFieldInteger {
				return false
			}
		}
		if field.Type == capmodel.ConfigurationFieldInteger {
			return nativeRemoteQuantityValid(field)
		}
		if field.Type != capmodel.ConfigurationFieldSingleSelect && field.Type != capmodel.ConfigurationFieldMultiSelect {
			return false
		}
		for _, option := range field.Options {
			if !nativePositiveID(option.OptionToken) || len(option.Children) != 0 {
				return false
			}
		}
		return len(field.Options) > 0
	case "custom":
		if len(parts) != 2 {
			return false
		}
		switch field.Type {
		case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldSingleSelect:
			return true
		}
	}
	return false
}
func nativeRemoteQuantityValid(field channelcommercecap.ConfigurationField) bool {
	minimum, err := strconv.ParseInt(field.Constraints.Minimum, 10, 64)
	maximum, maxErr := strconv.ParseInt(field.Constraints.Maximum, 10, 64)
	return err == nil && maxErr == nil && minimum >= 0 && maximum >= minimum && strconv.FormatInt(minimum, 10) == field.Constraints.Minimum && strconv.FormatInt(maximum, 10) == field.Constraints.Maximum && len(field.Options) == 0
}
