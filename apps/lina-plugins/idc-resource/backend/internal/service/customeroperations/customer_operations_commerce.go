// NativeCommercial is a customer adapter for core-owned prices and settlement.
// It shares the existing owner services and never sends a provider write.
package customeroperations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type NativeCommercial struct {
	management *Service
	owner      servercommercialcap.Service
}

func NewCommercial(management *Service, owner servercommercialcap.Service) (*NativeCommercial, error) {
	if management == nil || owner == nil {
		return nil, ErrUnavailable
	}
	return &NativeCommercial{management: management, owner: owner}, nil
}

type CommercialQuoteCommand struct {
	ServerNo, Action, PackageNo, TargetQuoteNo, IdempotencyKey string
	Parameters                                                 json.RawMessage
}

type CommercialCheckoutCommand struct {
	ServerNo, QuoteNo, IdempotencyKey, ProofToken string
	Confirmed                                     bool
}

func (s *NativeCommercial) TrafficPackages(ctx context.Context, serverNo string) ([]servercommercialcap.TrafficPackage, error) {
	target, err := s.management.CommercialTarget(ctx, serverNo)
	if err != nil {
		return nil, err
	}
	return s.owner.ListTrafficPackages(ctx, target)
}

func (s *NativeCommercial) Quote(ctx context.Context, command CommercialQuoteCommand) (*servercommercialcap.Quote, error) {
	if !keyPattern.MatchString(command.IdempotencyKey) || len(command.IdempotencyKey) < 8 {
		return nil, ErrInvalid
	}
	action := serveroperationresourcecap.Action(command.Action)
	switch action {
	case serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionTrafficAdd, serveroperationresourcecap.ActionChangeIP, serveroperationresourcecap.ActionResize:
	default:
		return nil, ErrInvalid
	}
	if len(command.Parameters) > 16384 || len(command.PackageNo) > 40 || len(command.TargetQuoteNo) > 40 {
		return nil, ErrInvalid
	}
	if (action != serveroperationresourcecap.ActionResize && command.TargetQuoteNo != "") || (action != serveroperationresourcecap.ActionTrafficAdd && command.PackageNo != "") {
		return nil, ErrInvalid
	}
	parameters := command.Parameters
	if len(parameters) == 0 {
		parameters = json.RawMessage(`{}`)
	}
	if action == serveroperationresourcecap.ActionResize || action == serveroperationresourcecap.ActionChangeIP || action == serveroperationresourcecap.ActionTrafficAdd {
		fields, err := scalarObject(parameters)
		if err != nil || len(fields) != 0 {
			return nil, ErrInvalid
		}
	}
	target, err := s.management.CommercialTarget(ctx, command.ServerNo)
	if err != nil {
		return nil, err
	}
	request := servercommercialcap.QuoteRequest{Target: target, Action: action, Parameters: parameters, PackageNo: command.PackageNo, IdempotencyKey: command.IdempotencyKey}
	if action == serveroperationresourcecap.ActionResize {
		if command.TargetQuoteNo == "" {
			return nil, ErrInvalid
		}
		request.Resize = &servercommercialcap.ResizeInput{TargetQuoteNo: command.TargetQuoteNo}
	}
	return s.owner.Quote(ctx, request)
}

func (s *NativeCommercial) PrepareCheckout(ctx context.Context, command CommercialCheckoutCommand) (*customerauthcap.SessionConfirmation, error) {
	intent, _, err := s.checkoutIntent(ctx, command)
	if err != nil {
		return nil, err
	}
	if !commercialHighRisk(intent.Action) {
		return &customerauthcap.SessionConfirmation{}, nil
	}
	digest, err := commercialConfirmationDigest(command, intent)
	if err != nil {
		return nil, err
	}
	return s.management.confirmation.PrepareCurrentConfirmation(ctx, digest)
}

func (s *NativeCommercial) Checkout(ctx context.Context, command CommercialCheckoutCommand) (*servercommercialcap.Purchase, error) {
	if !command.Confirmed || len(command.ProofToken) > 4096 || !validCommercialCheckout(command) {
		return nil, ErrInvalid
	}
	if existing, err := s.readPurchase(ctx, command); err != nil || existing != nil {
		return existing, err
	}
	intent, target, err := s.checkoutIntent(ctx, command)
	if err != nil {
		return s.recoverPurchase(ctx, command, err)
	}
	digest, err := commercialConfirmationDigest(command, intent)
	if err != nil {
		return nil, err
	}
	var result *servercommercialcap.Purchase
	err = s.management.database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		var checkoutErr error
		result, checkoutErr = s.owner.Checkout(txctx, servercommercialcap.CheckoutRequest{QuoteNo: command.QuoteNo, IdempotencyKey: command.IdempotencyKey, Target: target})
		if checkoutErr != nil {
			return checkoutErr
		}
		if result == nil {
			return ErrUnavailable
		}
		// Core resolves duplicate intent before proof consumption. New checkout,
		// inventory and proof all roll back together if exact confirmation fails.
		if !result.Reused && commercialHighRisk(intent.Action) {
			return s.management.confirmation.ConsumeCurrentConfirmation(txctx, digest, command.ProofToken)
		}
		return nil
	})
	if err != nil {
		return s.recoverPurchase(ctx, command, err)
	}
	return result, nil
}

func (s *NativeCommercial) checkoutIntent(ctx context.Context, command CommercialCheckoutCommand) (*servercommercialcap.CheckoutIntent, servercommercialcap.TargetSnapshot, error) {
	var target servercommercialcap.TargetSnapshot
	if !validCommercialCheckout(command) {
		return nil, target, ErrInvalid
	}
	target, err := s.management.CommercialTarget(ctx, command.ServerNo)
	if err != nil {
		return nil, target, err
	}
	intent, err := s.owner.ReadCheckout(ctx, servercommercialcap.CheckoutRequest{QuoteNo: command.QuoteNo, IdempotencyKey: command.IdempotencyKey, Target: target})
	if err != nil {
		return nil, target, err
	}
	if intent == nil || intent.QuoteNo != command.QuoteNo || intent.EntitlementNo != target.EntitlementNo || intent.LocalResourceNo != target.LocalResourceNo {
		return nil, target, ErrUnavailable
	}
	return intent, target, nil
}

func commercialHighRisk(action serveroperationresourcecap.Action) bool {
	return action == serveroperationresourcecap.ActionChangeIP || action == serveroperationresourcecap.ActionResize
}

func commercialConfirmationDigest(command CommercialCheckoutCommand, intent *servercommercialcap.CheckoutIntent) (string, error) {
	payload, err := json.Marshal(struct {
		Scope, ServerNo, QuoteNo, Key, Currency, Action string
		Amount                                          int64
	}{"native-commercial-confirmation/v1", command.ServerNo, intent.QuoteNo, command.IdempotencyKey, intent.CurrencyCode, string(intent.Action), intent.TotalAmountMinor})
	if err != nil {
		return "", ErrInvalid
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func validCommercialCheckout(command CommercialCheckoutCommand) bool {
	return keyPattern.MatchString(command.IdempotencyKey) && len(command.IdempotencyKey) >= 8 && command.QuoteNo != "" && len(command.QuoteNo) <= 40
}

func (s *NativeCommercial) readPurchase(ctx context.Context, command CommercialCheckoutCommand) (*servercommercialcap.Purchase, error) {
	owned, err := s.management.ownedRoute(ctx, command.ServerNo, false)
	if err != nil {
		return nil, err
	}
	return s.owner.ReadPurchase(ctx, servercommercialcap.PurchaseReadRequest{QuoteNo: command.QuoteNo, IdempotencyKey: command.IdempotencyKey, EntitlementNo: owned.EntitlementNo, CustomerNo: owned.CustomerNo})
}

func (s *NativeCommercial) recoverPurchase(ctx context.Context, command CommercialCheckoutCommand, cause error) (*servercommercialcap.Purchase, error) {
	if existing, err := s.readPurchase(ctx, command); err == nil && existing != nil {
		return existing, nil
	}
	return nil, cause
}
