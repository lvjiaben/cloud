// This file adapts the core-owned dynamic configuration without a customer cart
// or any supplier write. Current ownership is resolved before calling core.
package customeroperations

import (
	"context"
	"encoding/json"
	"time"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

// ConfigurationCommand carries only opaque typed form values and selectors.
// Nil Values reads the current configuration; an empty slice is an empty edit.
type ConfigurationCommand struct {
	ServerNo, ConfigurationRevision, BillingTermToken string
	Values                                            []servercommercialcap.TypedPurchaseValue
}

// Configuration returns the current product-defined form or evaluates a full
// edit. It never interprets supplier field names or calculates its own price.
func (s *NativeCommercial) Configuration(ctx context.Context, command ConfigurationCommand) (*servercommercialcap.ConfigurationContext, error) {
	reader, ok := s.owner.(servercommercialcap.ConfigurationContextReader)
	if !ok {
		return nil, ErrUnavailable
	}
	if !validConfigurationCommand(command, false) {
		return nil, ErrInvalid
	}
	// One budget covers both live resource observation and product evaluation;
	// nested candidate lookups cannot each restart a fresh request timeout.
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	target, err := s.management.CommercialTarget(ctx, command.ServerNo)
	if err != nil {
		return nil, err
	}
	result, err := reader.ReadConfigurationContext(ctx, servercommercialcap.ConfigurationContextRequest{Target: target, ConfigurationRevision: command.ConfigurationRevision, Values: command.Values})
	if ctx.Err() != nil {
		return nil, ErrConfigurationTimeout
	}
	return result, err
}

// QuoteTarget freezes a complete edit through the existing commercial owner;
// the separate commercial Quote remains authoritative for any final difference.
func (s *NativeCommercial) QuoteTarget(ctx context.Context, command ConfigurationCommand) (*servercommercialcap.TargetQuote, error) {
	if !validConfigurationCommand(command, true) {
		return nil, ErrInvalid
	}
	target, err := s.management.CommercialTarget(ctx, command.ServerNo)
	if err != nil {
		return nil, err
	}
	return s.owner.QuoteTarget(ctx, servercommercialcap.StructuredTargetInput{Target: target, ConfigurationRevision: command.ConfigurationRevision, BillingTermToken: command.BillingTermToken, Values: command.Values})
}

// Bound the full edit before any supplier observation or core compilation.
func validConfigurationCommand(command ConfigurationCommand, quote bool) bool {
	if len(command.Values) > 100 || len(command.ConfigurationRevision) > 128 || len(command.BillingTermToken) > 4096 {
		return false
	}
	if command.Values != nil && command.ConfigurationRevision == "" {
		return false
	}
	if quote && (command.Values == nil || command.ConfigurationRevision == "" || command.BillingTermToken == "") {
		return false
	}
	data, err := json.Marshal(command.Values)
	return err == nil && len(data) <= 16384
}
