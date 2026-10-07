package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const maximumAssistedDiscountMinor int64 = 9_000_000_000_000_000

var assistedCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type normalizedAssistedQuoteCreate struct {
	AssistedQuoteCreateInput
	idempotencyDigest string
	requestDigest     string
}

type resolvedAssistedQuoteItem struct {
	requestItemNo string
	productNameZh string
	productNameEn string
	resolved      *catalog.PurchaseConfiguration
	numbers       snapshotNumbers
	input         AssistedQuoteItemInput
}

func (s *assistedQuoteServiceImpl) CreateAssistedQuote(
	ctx context.Context,
	input AssistedQuoteCreateInput,
) (*AssistedQuoteRequest, error) {
	normalized, err := normalizeAssistedQuoteCreate(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorizer.AuthorizeAssistedQuoteCreate(ctx, normalized.CustomerNo)
	if err != nil {
		return nil, err
	}
	existing, err := loadAssistedQuoteByIdempotency(
		ctx, s.database, actor, normalized.idempotencyDigest, false,
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	if existing != nil {
		return s.projectAssistedCreateReplay(ctx, existing, actor, normalized.requestDigest)
	}
	resolved, err := s.resolveAssistedQuoteItems(ctx, normalized.Items)
	if err != nil {
		return nil, err
	}
	requestNo, err := newOpaqueNumber(s.random, "AQR_")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	var persisted *entity.CommerceAssistedQuoteRequest
	reused := false
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := lockAssistedQuoteIdempotency(
			ctx, tx, actor, normalized.idempotencyDigest,
		); err != nil {
			return err
		}
		persisted, err = loadAssistedQuoteByIdempotency(
			ctx, s.database, actor, normalized.idempotencyDigest, true,
		)
		if err != nil {
			return err
		}
		if persisted != nil {
			if persisted.RequestDigest != normalized.requestDigest {
				return bizerr.NewCode(CodeAssistedQuoteConflict)
			}
			reused = true
			return nil
		}
		customerRow, lockErr := lockAssistedQuoteCustomer(ctx, tx, normalized.CustomerNo)
		if lockErr != nil {
			return lockErr
		}
		if normalized.ContractPolicyCode == contractpolicy.PolicyRequired {
			_, contractErr := s.commerce.contractPolicy.Resolve(ctx, contractpolicy.ResolveInput{
				CustomerType:   contracttemplatecap.CustomerType(customerRow.CustomerTypeCode),
				EnterpriseRule: contractpolicy.Rule{Policy: contractpolicy.PolicyNone},
				ProductRules:   []contractpolicy.Rule{{Policy: contractpolicy.PolicyInherit}},
				OrderOverride: contractpolicy.Rule{
					Policy: normalized.ContractPolicyCode, TemplateNo: normalized.ContractTemplateNo,
				},
			})
			if contractErr != nil {
				return mapContractPolicyResolveError(contractErr)
			}
		}
		paymentDueSeconds := normalized.PaymentDueSeconds
		if normalized.PaymentTermCode == AssistedPaymentTermStandardPrepay {
			window, policyErr := loadCheckoutPaymentWindow(ctx, tx)
			if policyErr != nil {
				return policyErr
			}
			paymentDueSeconds = int(window / time.Second)
		}
		now, nowErr := checkoutDatabaseNow(ctx, tx)
		if nowErr != nil {
			return nowErr
		}
		expiresAt := now.Add(time.Duration(normalized.ValiditySeconds) * time.Second)
		requestRow := do.CommerceAssistedQuoteRequest{
			TenantId: idccap.DefaultTenantID, RequestNo: requestNo,
			CustomerNo: normalized.CustomerNo, CurrencyCode: normalized.CurrencyCode,
			StatusCode: string(AssistedQuoteStatusPendingApproval), ItemCount: len(resolved),
			ManualDiscountMinor: normalized.ManualDiscountMinor,
			PaymentTermCode:     string(normalized.PaymentTermCode), PaymentDueSeconds: paymentDueSeconds,
			Reason: normalized.Reason, InternalNote: normalized.InternalNote,
			IdempotencyDigest: normalized.idempotencyDigest, RequestDigest: normalized.requestDigest,
			ContractOverridePolicyCode: string(normalized.ContractPolicyCode),
			ExpiresAt:                  &expiresAt, RequestedBy: actor, Version: 1,
			CreatedAt: &now, UpdatedAt: &now,
		}
		if normalized.ContractTemplateNo != "" {
			requestRow.ContractOverrideTemplateNo = normalized.ContractTemplateNo
		}
		if _, err = tx.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
			Data(requestRow).Insert(); err != nil {
			return gerror.Wrap(err, "insert assisted quote request")
		}
		itemRows := make([]do.CommerceAssistedQuoteRequestItem, 0, len(resolved))
		quantityRows := make([]map[string]any, 0, len(resolved))
		for index, item := range resolved {
			if err = insertGenericPurchaseSnapshot(ctx, s.database, item.resolved, item.numbers.snapshotNo); err != nil {
				return err
			}
			itemDigest, digestErr := quoteContentDigest(struct {
				RequestNo, RequestItemNo, ProductNo, VersionNo, BillingRuleNo, SnapshotNo string
				Quantity, PurchaseCycles                                                  int
			}{
				requestNo, item.requestItemNo, item.resolved.ProductNo, item.resolved.VersionNo,
				item.resolved.BillingRuleNo, item.numbers.snapshotNo,
				item.input.Quantity, item.input.Purchase.PurchaseCycles,
			})
			if digestErr != nil {
				return digestErr
			}
			itemRows = append(itemRows, do.CommerceAssistedQuoteRequestItem{
				TenantId: idccap.DefaultTenantID, RequestItemNo: item.requestItemNo,
				RequestNo: requestNo, CurrencyCode: normalized.CurrencyCode, LineNumber: index + 1,
				ProductNo: item.resolved.ProductNo, VersionNo: item.resolved.VersionNo,
				BillingRuleNo:           item.resolved.BillingRuleNo,
				ConfigurationSnapshotNo: item.numbers.snapshotNo,
				ProductNameZhCn:         item.productNameZh, ProductNameEnUs: item.productNameEn,
				PurchaseCycles: item.input.Purchase.PurchaseCycles,
				ContentDigest:  itemDigest, CreatedAt: &now,
			})
			quantityDigest, digestErr := quoteContentDigest(struct {
				RequestItemNo string
				Quantity      int
			}{item.requestItemNo, item.input.Quantity})
			if digestErr != nil {
				return digestErr
			}
			quantityRows = append(quantityRows, map[string]any{"tenant_id": idccap.DefaultTenantID, "request_item_no": item.requestItemNo, "quantity": item.input.Quantity, "content_digest": quantityDigest})
		}
		if _, err = tx.Model(dao.CommerceAssistedQuoteRequestItem.Table()).Ctx(ctx).
			Data(itemRows).Insert(); err != nil {
			return gerror.Wrap(err, "insert assisted quote request items")
		}
		if _, err = tx.Model("plugin_idc_core_commerce_assisted_quote_item_quantity").Ctx(ctx).Data(quantityRows).Insert(); err != nil {
			return gerror.Wrap(err, "insert assisted generic quantities")
		}
		if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "commerce.assisted-quote.create",
			IdempotencyKey:   normalized.IdempotencyKey,
			OperationCode:    "commerce.assisted-quote.create", TargetType: "assisted_quote_request",
			TargetNo: requestNo, CustomerNo: normalized.CustomerNo,
			ResultCode: "pending_approval", ReasonCode: "staff_assisted_quote_requested",
			RiskCode: "high", RequiredPermission: permissionAssistedQuoteCreate,
			AfterSnapshot: map[string]any{
				"requestNo": requestNo, "currencyCode": normalized.CurrencyCode,
				"itemCount": len(resolved), "manualDiscountMinor": normalized.ManualDiscountMinor,
				"paymentTermCode":   normalized.PaymentTermCode,
				"paymentDueSeconds": paymentDueSeconds, "expiresAt": expiresAt,
				"contractPolicyCode": normalized.ContractPolicyCode,
				"contractTemplateNo": normalized.ContractTemplateNo,
				"customerVersion":    customerRow.Version,
			},
		}); err != nil {
			return err
		}
		persisted = &entity.CommerceAssistedQuoteRequest{
			RequestNo: requestNo, RequestDigest: normalized.requestDigest,
		}
		return nil
	})
	if err != nil {
		return nil, mapAssistedQuoteWriteError(err)
	}
	if persisted == nil {
		return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
	}
	if reused {
		return s.projectAssistedCreateReplay(ctx, persisted, actor, normalized.requestDigest)
	}
	request, err := loadAssistedQuoteRequest(ctx, s.database, requestNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "reload assisted quote request"), CodeAssistedQuoteUnavailable)
	}
	if request == nil {
		return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
	}
	views, err := loadAssistedQuoteProjections(ctx, s.database, []entity.CommerceAssistedQuoteRequest{*request}, actor, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	return &views[0], nil
}

func normalizeAssistedQuoteCreate(input AssistedQuoteCreateInput) (normalizedAssistedQuoteCreate, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.Reason = strings.TrimSpace(input.Reason)
	input.InternalNote = strings.TrimSpace(input.InternalNote)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.ContractTemplateNo = strings.TrimSpace(input.ContractTemplateNo)
	if input.ContractPolicyCode == "" {
		input.ContractPolicyCode = contractpolicy.PolicyInherit
	}
	rule, contractErr := contractpolicy.NormalizeRule(contractpolicy.Rule{
		Policy: input.ContractPolicyCode, TemplateNo: input.ContractTemplateNo,
	}, true)
	if contractErr != nil {
		return normalizedAssistedQuoteCreate{}, bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	input.ContractPolicyCode = rule.Policy
	input.ContractTemplateNo = rule.TemplateNo
	if !validAssistedQuoteCreateHeader(input) || !validAssistedPaymentTerm(input) {
		return normalizedAssistedQuoteCreate{}, bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	items, err := normalizeAssistedQuoteItems(input.Items, input.CurrencyCode)
	if err != nil {
		return normalizedAssistedQuoteCreate{}, err
	}
	input.Items = items
	requestDigest, err := quoteContentDigest(struct {
		CustomerNo, CurrencyCode string
		ManualDiscountMinor      int64
		PaymentTermCode          AssistedPaymentTerm
		PaymentDueSeconds        int
		Reason, InternalNote     string
		ContractPolicyCode       contractpolicy.Policy
		ContractTemplateNo       string
		ValiditySeconds          int
		Items                    []AssistedQuoteItemInput
	}{
		input.CustomerNo, input.CurrencyCode, input.ManualDiscountMinor,
		input.PaymentTermCode, input.PaymentDueSeconds, input.Reason,
		input.InternalNote, input.ContractPolicyCode, input.ContractTemplateNo,
		input.ValiditySeconds, input.Items,
	})
	if err != nil {
		return normalizedAssistedQuoteCreate{}, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	idempotencyHash := sha256.Sum256([]byte("assisted-quote-create/v1\x00" + input.IdempotencyKey))
	return normalizedAssistedQuoteCreate{
		AssistedQuoteCreateInput: input,
		idempotencyDigest:        hex.EncodeToString(idempotencyHash[:]), requestDigest: requestDigest,
	}, nil
}

func validAssistedQuoteCreateHeader(input AssistedQuoteCreateInput) bool {
	reasonLength := utf8.RuneCountInString(input.Reason)
	return validAssistedIdentifier(input.CustomerNo) && assistedCurrencyPattern.MatchString(input.CurrencyCode) &&
		input.ManualDiscountMinor >= 0 && input.ManualDiscountMinor <= maximumAssistedDiscountMinor &&
		reasonLength >= 10 && reasonLength <= 1000 && utf8.ValidString(input.Reason) &&
		utf8.RuneCountInString(input.InternalNote) <= 2000 && utf8.ValidString(input.InternalNote) &&
		checkoutIdempotencyPattern.MatchString(input.IdempotencyKey) &&
		input.ValiditySeconds >= 900 && input.ValiditySeconds <= 604800 &&
		len(input.Items) >= 1 && len(input.Items) <= maximumAssistedQuoteItems
}

func validAssistedPaymentTerm(input AssistedQuoteCreateInput) bool {
	if input.PaymentTermCode == AssistedPaymentTermStandardPrepay {
		return input.PaymentDueSeconds == 0
	}
	return input.PaymentTermCode == AssistedPaymentTermExtendedPrepay &&
		input.PaymentDueSeconds >= 86401 && input.PaymentDueSeconds <= 604800
}

func normalizeAssistedQuoteItems(
	inputs []AssistedQuoteItemInput,
	currencyCode string,
) ([]AssistedQuoteItemInput, error) {
	items := append([]AssistedQuoteItemInput(nil), inputs...)
	for index := range items {
		if err := normalizeAssistedQuoteItem(&items[index], currencyCode); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func normalizeAssistedQuoteItem(item *AssistedQuoteItemInput, currencyCode string) error {
	purchase := &item.Purchase
	purchase.PublicSlug = strings.ToLower(strings.TrimSpace(purchase.PublicSlug))
	purchase.ConfigurationRevision = strings.TrimSpace(purchase.ConfigurationRevision)
	purchase.BillingTermToken = strings.TrimSpace(purchase.BillingTermToken)
	purchase.CurrencyCode = strings.TrimSpace(purchase.CurrencyCode)
	if item.Quantity < 1 || item.Quantity > 1000 || purchase.CurrencyCode != currencyCode ||
		purchase.PurchaseCycles < 1 || purchase.PurchaseCycles > 120 || purchase.PublicSlug == "" ||
		purchase.ConfigurationRevision == "" || purchase.BillingTermToken == "" {
		return bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	return nil
}

func (s *assistedQuoteServiceImpl) resolveAssistedQuoteItems(
	ctx context.Context,
	inputs []AssistedQuoteItemInput,
) ([]resolvedAssistedQuoteItem, error) {
	result := make([]resolvedAssistedQuoteItem, 0, len(inputs))
	references := make([]catalog.PurchaseConfigurationReference, 0, len(inputs))
	for index, input := range inputs {
		resolved, err := s.catalog.ResolvePurchaseConfiguration(ctx, input.Purchase)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeAssistedQuoteInvalid)
		}
		if resolved == nil {
			return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
		}
		requestItemNo, err := newOpaqueNumber(s.random, "AQI_")
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
		}
		numbers, err := s.commerce.newSnapshotNumbers()
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
		}
		referenceNo := "AQREF_" + string(rune('A'+index))
		references = append(references, catalog.PurchaseConfigurationReference{
			ReferenceNo: referenceNo, ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo,
			BillingRuleNo: resolved.BillingRuleNo,
		})
		result = append(result, resolvedAssistedQuoteItem{
			requestItemNo: requestItemNo, resolved: resolved, numbers: numbers,
			input: input,
		})
	}
	states, err := s.catalog.RevalidatePurchaseConfigurations(ctx, references)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteInvalid)
	}
	if len(states) != len(result) {
		return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
	}
	byReference := make(map[string]catalog.PurchaseConfigurationState, len(states))
	for _, state := range states {
		if state.IssueCode != "" || state.ProductNameZhCN == "" || state.ProductNameEnUS == "" {
			return nil, bizerr.NewCode(CodeAssistedQuoteInvalid)
		}
		if _, duplicate := byReference[state.ReferenceNo]; duplicate {
			return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
		}
		byReference[state.ReferenceNo] = state
	}
	for index := range result {
		state, ok := byReference[references[index].ReferenceNo]
		if !ok {
			return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
		}
		result[index].productNameZh = state.ProductNameZhCN
		result[index].productNameEn = state.ProductNameEnUS
	}
	return result, nil
}

func lockAssistedQuoteIdempotency(
	ctx context.Context,
	tx gdb.TX,
	actor int,
	digest string,
) error {
	_, err := tx.Ctx(ctx).GetValue(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`,
		"idc-assisted-quote:"+strconv.Itoa(actor)+":"+digest)
	return gerror.Wrap(err, "lock assisted quote idempotency scope")
}

func loadAssistedQuoteByIdempotency(
	ctx context.Context,
	database gdb.DB,
	actor int,
	digest string,
	lock bool,
) (*entity.CommerceAssistedQuoteRequest, error) {
	var result *entity.CommerceAssistedQuoteRequest
	model := database.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).Where(
		do.CommerceAssistedQuoteRequest{
			TenantId: idccap.DefaultTenantID, RequestedBy: actor, IdempotencyDigest: digest,
		},
	)
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&result); err != nil {
		return nil, gerror.Wrap(err, "load assisted quote idempotency result")
	}
	return result, nil
}

func lockAssistedQuoteCustomer(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
) (*entity.CustomerSubject, error) {
	var result *entity.CustomerSubject
	if err := tx.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	}).LockUpdate().Scan(&result); err != nil {
		return nil, gerror.Wrap(err, "lock assisted quote customer")
	}
	if result == nil || result.StatusCode != string(customer.StatusActive) ||
		result.AnonymizedAt != nil || result.Version < 1 {
		return nil, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	return result, nil
}

func (s *assistedQuoteServiceImpl) projectAssistedCreateReplay(
	ctx context.Context,
	request *entity.CommerceAssistedQuoteRequest,
	actor int,
	requestDigest string,
) (*AssistedQuoteRequest, error) {
	if request.RequestDigest != requestDigest {
		return nil, bizerr.NewCode(CodeAssistedQuoteConflict)
	}
	views, err := loadAssistedQuoteProjections(
		ctx, s.database, []entity.CommerceAssistedQuoteRequest{*request}, actor, true,
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	return &views[0], nil
}

func mapAssistedQuoteWriteError(err error) error {
	if bizerr.Is(err, CodeAssistedQuoteInvalid) || bizerr.Is(err, CodeAssistedQuoteNotAccessible) ||
		bizerr.Is(err, CodeAssistedQuoteConflict) || bizerr.Is(err, CodeAssistedQuoteApprovalRequired) ||
		bizerr.Is(err, CodeQuoteNotAvailable) || bizerr.Is(err, CodeInvalid) ||
		bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) ||
		bizerr.Is(err, CodeContractPolicyInvalid) ||
		bizerr.Is(err, CodeContractTemplateUnavailable) ||
		bizerr.Is(err, CodeContractTemplateConflict) {
		return err
	}
	return bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
}
