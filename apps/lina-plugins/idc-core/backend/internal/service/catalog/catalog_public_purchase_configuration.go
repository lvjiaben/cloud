// This file projects frozen generic customer configuration through opaque
// public tokens and reuses the shared contract evaluator before quoting.

package catalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const maximumPublicStructuredConfigurationValues = 128

// PublicStructuredConfigurationReader is the anonymous generic product
// configuration boundary. It never exposes plugin field keys or option values.
type PublicStructuredConfigurationReader interface {
	GetPurchaseConfiguration(context.Context, string) (*PublicPurchaseConfigurationView, error)
	EvaluatePurchaseConfiguration(context.Context, PublicPurchaseConfigurationInput) (*PublicPurchaseConfigurationEvaluation, error)
	QuotePurchaseConfiguration(context.Context, PublicPurchaseConfigurationQuoteInput) (*PublicPurchaseConfigurationQuote, error)
}

// FrozenPurchaseConfiguration is compiler-produced internal evidence. Its raw
// keys and JSON documents are never part of an HTTP response.
type FrozenPurchaseConfiguration struct {
	ProductNo                      string
	VersionNo                      string
	ConfigurationRevision          string
	InterfaceInstanceNo            string
	InterfaceConfigurationRevision int64
	CustomerDeclarationDigest      string
	CandidateSnapshotDigest        string
	CustomerValueDigest            string
	PriceSelectionDigest           string
	CandidateSnapshotJSON          []byte
	TypedValueSnapshotJSON         []byte
	FileFields                     []purchaseconfigurationfile.VerifyInput
}

// PurchaseConfigurationFreezer is the trusted account transaction boundary.
// Callers provide opaque browser values; this service returns only output of
// the same thirteen-type compiler used by public evaluation and quoting.
type PurchaseConfigurationFreezer interface {
	FreezePurchaseConfiguration(context.Context, PublicPurchaseConfigurationInput) (*FrozenPurchaseConfiguration, error)
}

func PurchaseConfigurationFreezerFromPublicReader(reader PublicCatalogReader) (PurchaseConfigurationFreezer, error) {
	service, ok := reader.(PurchaseConfigurationFreezer)
	if !ok || service == nil {
		return nil, gerror.New("public catalog reader does not provide purchase configuration freezing")
	}
	return service, nil
}

// PublicPurchaseNewBusinessAvailability reuses the automation owner's exact
// plugin, release, binding, and interface lifecycle admission decision.
type PublicPurchaseNewBusinessAvailability interface {
	CheckPublicPurchaseAvailability(context.Context, string) (bool, error)
}

// PublicPurchaseDynamicCandidateResolver reads candidate choices from their owner.
// External calls retain provider admission; local projections use stored bindings.
// Missing or disabled local bindings fail closed; purchase admission is checked separately.
type PublicPurchaseDynamicCandidateResolver interface {
	ResolvePublicPurchaseCandidates(
		context.Context,
		string,
		*capmodel.ConfigurationSchemaDeclaration,
		[]configurationcontract.ConfigurationValue,
	) (configurationcontract.ResolvedCandidates, error)
}

type PublicPurchaseConfigurationRuntime interface {
	PublicPurchaseNewBusinessAvailability
	PublicPurchaseDynamicCandidateResolver
}

// StructuredConfigurationReaderFromPublicReader narrows startup composition
// without widening unrelated anonymous catalog consumers.
func StructuredConfigurationReaderFromPublicReader(reader PublicCatalogReader) (PublicStructuredConfigurationReader, error) {
	service, ok := reader.(PublicStructuredConfigurationReader)
	if !ok || service == nil {
		return nil, gerror.New("public catalog reader does not provide structured configuration")
	}
	return service, nil
}

// PublicLocalizedText is a safe bilingual public label.
type PublicLocalizedText struct {
	ZhCN string
	EnUS string
}

// PublicBillingLabelResources is a startup-validated bilingual projection of
// the plugin i18n bundle. Keeping the resource lookup outside request input
// prevents raw i18n keys or untranslated internal codes from reaching users.
type PublicBillingLabelResources map[string]PublicLocalizedText

// BindPublicBillingLabelResources attaches the packed plugin resources to the
// concrete public reader without widening the public catalog contract.
func BindPublicBillingLabelResources(reader PublicCatalogReader, resources PublicBillingLabelResources) error {
	service, ok := reader.(*publicCatalogReaderImpl)
	if !ok || service == nil || len(resources) == 0 {
		return gerror.New("public catalog billing label resources are unavailable")
	}
	required := []string{"charge.one_time", "period.hour", "period.day", "period.month", "period.year"}
	cloned := make(PublicBillingLabelResources, len(required))
	for _, key := range required {
		value := resources[key]
		if strings.TrimSpace(value.ZhCN) == "" || strings.TrimSpace(value.EnUS) == "" {
			return gerror.New("public catalog billing label resource is incomplete")
		}
		cloned[key] = value
	}
	service.billingLabels = cloned
	return nil
}

// PublicPurchaseConfigurationOption is one opaque static choice.
type PublicPurchaseConfigurationOption struct {
	OptionToken string
	Label       PublicLocalizedText
	Children    []PublicPurchaseConfigurationOption
}

// PublicPurchaseConfigurationConstraints contains only host-enforced limits.
type PublicPurchaseConfigurationConstraints struct {
	Format        string
	Minimum       string
	Maximum       string
	MinimumLength int
	MaximumLength int
}

// PublicPurchaseConfigurationUploadPolicy is the anonymous safe projection of
// host-enforced requirements. It never contains an upload address or token.
type PublicPurchaseConfigurationUploadPolicy struct {
	MaximumFiles       int
	MaximumFileBytes   int64
	MaximumTotalBytes  int64
	AllowedMIMETypes   []string
	AllowedExtensions  []string
	SafetyScanRequired bool
}

// PublicPurchaseConfigurationField contains no plugin key or raw candidate value.
type PublicPurchaseConfigurationField struct {
	Presentation string

	GroupZhCN      string
	GroupEnUS      string
	Unit           string
	Advanced       bool
	NumericLabels  map[string]*capmodel.LocalizedText
	DisplayControl string
	QuantityStep   string
	NumericChoices []string
	FieldToken     string
	Type           string
	Label          PublicLocalizedText
	Description    PublicLocalizedText
	Visible        bool
	Required       bool
	Disabled       bool
	Constraints    PublicPurchaseConfigurationConstraints
	Options        []PublicPurchaseConfigurationOption
	UploadPolicy   *PublicPurchaseConfigurationUploadPolicy
}

// PublicPurchaseConfigurationValue uses exactly one explicit typed arm.
type PublicPurchaseConfigurationValue struct {
	FieldToken          string
	Type                string
	TextValue           *string
	IntegerValue        *string
	DecimalValue        *string
	BooleanValue        *bool
	DateValue           *string
	DateTimeValue       *string
	OptionTokens        []string
	TreeOptionTokens    []string
	AddressOptionTokens []string
	FileTokens          []string
}

// PublicPurchaseConfigurationView is one current published configuration revision.
type PublicPurchaseConfigurationView struct {
	ServerLoginPolicy *PublicServerLoginPolicy

	Ready                 bool
	ConfigurationRevision string
	Fields                []PublicPurchaseConfigurationField
	Values                []PublicPurchaseConfigurationValue
}

// PublicPurchaseConfigurationIssue is stable and localizable by the client.
type PublicPurchaseConfigurationIssue struct {
	FieldToken string
	IssueCode  string
}

// PublicPurchaseConfigurationEvaluation includes authoritative runtime field state.
type PublicPurchaseConfigurationEvaluation struct {
	PublicPurchaseConfigurationView
	Valid  bool
	Issues []PublicPurchaseConfigurationIssue
}

// PublicPurchaseConfigurationInput is one complete interactive evaluation request.
type PublicPurchaseConfigurationInput struct {
	PublicSlug            string
	ConfigurationRevision string
	Values                []PublicPurchaseConfigurationValue
}

// PublicPurchaseConfigurationQuoteInput requests current explanatory prices.
type PublicPurchaseConfigurationQuoteInput struct {
	PublicPurchaseConfigurationInput
	CurrencyCode string
}

// PublicPurchaseConfigurationQuoteTerm is one core-owned published billing snapshot.
type PublicPurchaseConfigurationQuoteTerm struct {
	BillingTermToken      string
	Label                 PublicLocalizedText
	ChargeType            ChargeType
	PeriodUnit            PeriodUnit
	PeriodCount           int
	MinimumPurchaseCycles int
	Status                string
	SetupAmount           string
	SubtotalAmount        string
	TaxAmount             string
	TotalAmount           string
}

// PublicPurchaseConfigurationQuote is explanatory and intentionally not a locked quote.
type PublicPurchaseConfigurationQuote struct {
	ConfigurationRevision string
	CurrencyCode          string
	Terms                 []PublicPurchaseConfigurationQuoteTerm
}

type publicStructuredConfigurationTarget struct {
	ProductDeclaration string `orm:"product_declaration"`
	BindingPluginID    string `orm:"binding_plugin_id"`

	MerchantValues            string `orm:"merchant_values"`
	CustomerPresentation      string `orm:"customer_presentation"`
	ProductNo                 string `orm:"product_no"`
	VersionNo                 string `orm:"version_no"`
	ConfigurationVersion      int64  `orm:"configuration_version"`
	CustomerDeclaration       string `orm:"customer_declaration"`
	CustomerDeclarationDigest string `orm:"customer_declaration_digest"`
	HasAutomationBinding      bool   `orm:"has_automation_binding"`
}

type publicStructuredConfigurationSnapshot struct {
	Target   publicStructuredConfigurationTarget
	Revision string
	Schema   *capmodel.ConfigurationSchemaDeclaration
}

// GetPurchaseConfiguration returns the initial server-owned field state.
func (s *publicCatalogReaderImpl) GetPurchaseConfiguration(
	ctx context.Context,
	publicSlug string,
) (_ *PublicPurchaseConfigurationView, resultErr error) {
	defer func() {
		if resultErr == nil || !bizerr.Is(resultErr, CodePublicCatalogUnavailable) {
			return
		}
		codes := []string{}
		for cause := resultErr; cause != nil; cause = errors.Unwrap(cause) {
			if typed, ok := bizerr.As(cause); ok {
				codes = append(codes, typed.RuntimeCode())
			}
		}
		logger.Warningf(ctx, "public purchase configuration failed codes=%s", strings.Join(codes, ","))
	}()
	snapshot, err := s.loadStructuredConfigurationForEvaluation(ctx, publicSlug, "")
	if err != nil {
		return nil, err
	}
	evaluation, resolved, err := s.evaluateInteractiveStructuredConfiguration(ctx, snapshot, nil)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	view, err := s.projectStructuredConfiguration(snapshot, evaluation, resolved)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	return view, nil
}

// EvaluatePurchaseConfiguration returns authoritative state and stable issues.
func (s *publicCatalogReaderImpl) EvaluatePurchaseConfiguration(
	ctx context.Context,
	input PublicPurchaseConfigurationInput,
) (*PublicPurchaseConfigurationEvaluation, error) {
	snapshot, err := s.loadStructuredConfigurationForEvaluation(ctx, input.PublicSlug, input.ConfigurationRevision)
	if err != nil {
		return nil, err
	}
	values, err := s.decodePublicStructuredValues(snapshot, input.Values)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	if err = s.verifyStructuredFiles(ctx, snapshot, values); err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	evaluation, resolved, err := s.evaluateInteractiveStructuredConfiguration(ctx, snapshot, values)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	view, err := s.projectStructuredConfiguration(snapshot, evaluation, resolved)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	issues := make([]PublicPurchaseConfigurationIssue, 0, len(evaluation.Issues))
	for _, issue := range evaluation.Issues {
		token, tokenErr := s.revision.encodeStructuredField(snapshot.Revision, issue.FieldKey)
		if tokenErr != nil {
			return nil, bizerr.WrapCode(tokenErr, CodePublicCatalogUnavailable)
		}
		issues = append(issues, PublicPurchaseConfigurationIssue{FieldToken: token, IssueCode: issue.Code})
	}
	return &PublicPurchaseConfigurationEvaluation{
		PublicPurchaseConfigurationView: *view,
		Valid:                           evaluation.Valid, Issues: issues,
	}, nil
}

// FreezePurchaseConfiguration performs the authoritative ready-token check
// and emits immutable compiler evidence without consuming any file token.
func (s *publicCatalogReaderImpl) FreezePurchaseConfiguration(
	ctx context.Context,
	input PublicPurchaseConfigurationInput,
) (*FrozenPurchaseConfiguration, error) {
	snapshot, err := s.loadStructuredConfigurationForEvaluation(ctx, input.PublicSlug, input.ConfigurationRevision)
	if err != nil {
		return nil, err
	}
	values, err := s.decodePublicStructuredValues(snapshot, input.Values)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	if err = s.verifyStructuredFiles(ctx, snapshot, values); err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	// Freezing is an explicit purchase, never an interactive correction. A
	// stale parent/child pair must be rejected before producing sale evidence.
	evaluation, resolved, err := s.evaluateStructuredConfiguration(ctx, snapshot, values)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if !evaluation.Valid {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	compiled, err := configurationcontract.CompileResolved(
		snapshot.Schema, values, configurationcontract.SurfaceCustomer,
		authorizedDynamicMethods(snapshot.Schema), resolved,
	)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	candidateJSON, err := json.Marshal(resolved)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	var binding struct {
		InterfaceInstanceNo            string `orm:"interface_instance_no"`
		InterfaceConfigurationRevision int64  `orm:"interface_configuration_revision"`
	}
	if snapshot.Target.HasAutomationBinding {
		if err = s.database.GetScan(ctx, &binding, `
SELECT configuration.interface_instance_no,
CASE WHEN interface.interface_origin_code='resource' THEN
    (SELECT MAX(current_revision.revision)
     FROM plugin_idc_core_product_automation_interface_configuration_revision current_revision
     WHERE current_revision.tenant_id=configuration.tenant_id
       AND current_revision.interface_instance_no=configuration.interface_instance_no)
ELSE configuration.interface_configuration_revision END AS interface_configuration_revision
FROM plugin_idc_core_catalog_product_version_configuration configuration
JOIN plugin_idc_core_product_automation_interface_instance interface
  ON interface.tenant_id=configuration.tenant_id
 AND interface.interface_instance_no=configuration.interface_instance_no
WHERE configuration.tenant_id = 0 AND configuration.version_no = ?
LIMIT 1`, snapshot.Target.VersionNo); err != nil || binding.InterfaceInstanceNo == "" || binding.InterfaceConfigurationRevision < 1 {
			return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
		}
	}
	fileFields, err := frozenPublicFileInputs(snapshot, values)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	if err = s.recheckStructuredConfiguration(ctx, snapshot); err != nil {
		return nil, err
	}
	_, priceSelectionDigest, err := loadSelectedConfigurationPriceDimensions(ctx, s.database, snapshot.Target.VersionNo, values, snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	return &FrozenPurchaseConfiguration{
		ProductNo: snapshot.Target.ProductNo, VersionNo: snapshot.Target.VersionNo,
		ConfigurationRevision:          input.ConfigurationRevision,
		InterfaceInstanceNo:            binding.InterfaceInstanceNo,
		InterfaceConfigurationRevision: binding.InterfaceConfigurationRevision,
		CustomerDeclarationDigest:      frozenConfigurationDigest(compiled.DeclarationJSON),
		CandidateSnapshotDigest:        frozenConfigurationDigest(candidateJSON),
		CustomerValueDigest:            frozenConfigurationDigest(compiled.ValuesJSON),
		PriceSelectionDigest:           priceSelectionDigest,
		CandidateSnapshotJSON:          append([]byte(nil), candidateJSON...),
		TypedValueSnapshotJSON:         append([]byte(nil), compiled.ValuesJSON...),
		FileFields:                     fileFields,
	}, nil
}

func frozenConfigurationDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func frozenPublicFileInputs(snapshot *publicStructuredConfigurationSnapshot, values []configurationcontract.ConfigurationValue) ([]purchaseconfigurationfile.VerifyInput, error) {
	result := make([]purchaseconfigurationfile.VerifyInput, 0)
	for _, value := range values {
		if value.FileTokens == nil {
			continue
		}
		field := configurationFieldByKey(snapshot.Schema, value.FieldKey)
		if field == nil || field.Type != capmodel.ConfigurationFieldFile {
			return nil, gerror.New("frozen purchase file declaration is invalid")
		}
		policy := configurationUploadPolicy(snapshot.Schema, field.UploadPolicy)
		if policy == nil {
			return nil, gerror.New("frozen purchase file declaration is invalid")
		}
		result = append(result, purchaseconfigurationfile.VerifyInput{Target: &purchaseconfigurationfile.Target{
			ProductVersionNo: snapshot.Target.VersionNo, ConfigurationRevision: snapshot.Revision,
			FieldKey: field.Key, Policy: policy,
		}, FileTokens: append([]string(nil), value.FileTokens...)})
	}
	return result, nil
}

// QuotePurchaseConfiguration revalidates the complete configuration before
// reading only core-owned explicit currency integer price snapshots.
func (s *publicCatalogReaderImpl) QuotePurchaseConfiguration(
	ctx context.Context,
	input PublicPurchaseConfigurationQuoteInput,
) (*PublicPurchaseConfigurationQuote, error) {
	currency := strings.TrimSpace(input.CurrencyCode)
	if currency != input.CurrencyCode || !currencyCodePattern.MatchString(currency) {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	if err := s.admitCurrency(ctx, currency); err != nil {
		return nil, err
	}
	snapshot, err := s.loadStructuredConfigurationForEvaluation(ctx, input.PublicSlug, input.ConfigurationRevision)
	if err != nil {
		return nil, err
	}
	values, err := s.decodePublicStructuredValues(snapshot, input.Values)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	if err = s.verifyStructuredFiles(ctx, snapshot, values); err != nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	evaluation, _, err := s.evaluateStructuredConfiguration(ctx, snapshot, values)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	if !evaluation.Valid {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	selections, selectionDigest, err := loadSelectedConfigurationPriceDimensions(ctx, s.database, snapshot.Target.VersionNo, values, snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	source, err := loadPublicQuoteSource(ctx, s.database, snapshot.Target.VersionNo, currency)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	baseQuote, err := s.quoteStructuredPurchasePrices(ctx, snapshot.Target.VersionNo, snapshot.Revision, currency, source, configurationPriceSelectionMap(selections), values)
	if err != nil {
		return nil, err
	}
	if err = s.applyImportedPublicPrices(ctx, snapshot.Target.VersionNo, baseQuote); err != nil {
		return nil, err
	}
	terms := make([]PublicPurchaseConfigurationQuoteTerm, 0, len(baseQuote.Terms))
	for _, term := range baseQuote.Terms {
		if term.Status != PublicQuoteFinal && term.Status != PublicQuoteRequiresCustomerContext {
			continue
		}
		token, tokenErr := s.revision.encodeStructuredQuoteTerm(
			strings.ToLower(strings.TrimSpace(input.PublicSlug)), snapshot.Target.VersionNo, term.TermNo, selectionDigest,
		)
		if tokenErr != nil {
			return nil, bizerr.WrapCode(tokenErr, CodePublicQuoteUnavailable)
		}
		setup, subtotal := int64(0), int64(0)
		for _, line := range term.Lines {
			setup, err = addBillingAmount(setup, line.SetupFeeMinor)
			if err != nil {
				return nil, err
			}
			subtotal, err = addBillingAmount(subtotal, line.ChargeAmountMinor)
			if err != nil {
				return nil, err
			}
		}
		tax, total := int64(0), int64(0)
		if term.TaxAmountMinor != nil {
			tax = *term.TaxAmountMinor
		}
		if term.TotalAmountMinor != nil {
			total = *term.TotalAmountMinor
		}
		label, labelErr := publicBillingTermLabel(term, s.billingLabels)
		if labelErr != nil {
			return nil, bizerr.WrapCode(labelErr, CodePublicQuoteUnavailable)
		}
		terms = append(terms, PublicPurchaseConfigurationQuoteTerm{
			BillingTermToken: token, Label: label,
			ChargeType: term.ChargeType, PeriodUnit: term.PeriodUnit, PeriodCount: term.PeriodCount,
			MinimumPurchaseCycles: term.MinimumPurchaseCycles, Status: term.Status,
			SetupAmount: fmt.Sprintf("%d", setup), SubtotalAmount: fmt.Sprintf("%d", subtotal),
			TaxAmount: fmt.Sprintf("%d", tax), TotalAmount: fmt.Sprintf("%d", total),
		})
	}
	if len(terms) == 0 {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	if err = s.recheckStructuredConfigurationRevision(ctx, snapshot); err != nil {
		return nil, err
	}
	return &PublicPurchaseConfigurationQuote{
		ConfigurationRevision: snapshot.Revision, CurrencyCode: currency, Terms: terms,
	}, nil
}

func (s *publicCatalogReaderImpl) loadStructuredConfiguration(
	ctx context.Context,
	publicSlug string,
	expectedRevision string,
) (*publicStructuredConfigurationSnapshot, error) {
	return s.loadStructuredConfigurationSnapshot(ctx, publicSlug, expectedRevision, false)
}

// Interactive configuration is a database-backed price preview. Provider
// admission belongs to freezing/purchasing and actual provider execution.
func (s *publicCatalogReaderImpl) loadStructuredConfigurationForEvaluation(ctx context.Context, publicSlug, expectedRevision string) (*publicStructuredConfigurationSnapshot, error) {
	return s.loadStructuredConfigurationSnapshot(ctx, publicSlug, expectedRevision, true)
}

func (s *publicCatalogReaderImpl) loadStructuredConfigurationSnapshot(ctx context.Context, publicSlug, expectedRevision string, preview bool) (*publicStructuredConfigurationSnapshot, error) {
	slug := strings.ToLower(strings.TrimSpace(publicSlug))
	if !validPublicConfigurationSlug(slug) || s.revision == nil {
		return nil, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	var target publicStructuredConfigurationTarget
	if err := s.database.GetScan(ctx, &target, publicStructuredConfigurationTargetSQL, slug); errors.Is(err, sql.ErrNoRows) {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	} else if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if target.VersionNo == "" {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	}
	digest := target.CustomerDeclarationDigest
	if digest == "" {
		digest = "none"
	}
	revision, err := s.revision.encodeStructuredRevision(slug, target.VersionNo, target.ConfigurationVersion, digest)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if expectedRevision != "" && expectedRevision != revision {
		return nil, bizerr.NewCode(CodePublicConfigurationStale)
	}
	schema, err := configurationcontract.DecodeDeclaration(target.CustomerDeclaration)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	// Published schemas already freeze their declared methods. The resolver
	// checks the live exact binding before invoking those methods; passing nil
	// here would reject every valid published dynamic field before that check.
	if _, err = configurationcontract.ValidateSchema(schema, configurationcontract.SurfaceCustomer, authorizedDynamicMethods(schema)); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	snapshot := &publicStructuredConfigurationSnapshot{Target: target, Revision: revision, Schema: schema}
	if preview {
		// This query already checks published version, product and category state.
		return snapshot, nil
	}
	if err = s.recheckStructuredConfiguration(ctx, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// recheckStructuredConfiguration retains full admission for purchase freezing.
func (s *publicCatalogReaderImpl) recheckStructuredConfiguration(ctx context.Context, snapshot *publicStructuredConfigurationSnapshot) error {
	if err := s.recheckStructuredConfigurationRevision(ctx, snapshot); err != nil {
		return err
	}
	return s.ensurePublicPurchaseAvailability(ctx, snapshot.Target)
}

func (s *publicCatalogReaderImpl) recheckStructuredConfigurationRevision(ctx context.Context, snapshot *publicStructuredConfigurationSnapshot) error {
	current, err := s.database.GetValue(ctx, publicStructuredConfigurationStillCurrentSQL,
		snapshot.Target.VersionNo, snapshot.Target.ConfigurationVersion,
		snapshot.Target.CustomerDeclarationDigest,
	)
	if err != nil {
		return bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if !current.Bool() {
		return bizerr.NewCode(CodePublicConfigurationStale)
	}
	return nil
}

func (s *publicCatalogReaderImpl) ensurePublicPurchaseAvailability(
	ctx context.Context,
	target publicStructuredConfigurationTarget,
) error {
	if !target.HasAutomationBinding {
		return nil
	}
	if s.newBusinessAvailability == nil {
		return bizerr.NewCode(CodePublicCatalogUnavailable)
	}
	available, err := s.newBusinessAvailability.CheckPublicPurchaseAvailability(ctx, target.VersionNo)
	if err != nil {
		return bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if !available {
		return bizerr.NewCode(CodePublicCatalogNotFound)
	}
	return nil
}

func (s *publicCatalogReaderImpl) evaluateStructuredConfiguration(
	ctx context.Context,
	snapshot *publicStructuredConfigurationSnapshot,
	values []configurationcontract.ConfigurationValue,
) (*configurationcontract.Evaluation, configurationcontract.ResolvedCandidates, error) {
	presentations, err := configurationcontract.DecodePresentations(snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, nil, err
	}
	values, err = configurationcontract.ApplyHiddenDefaults(snapshot.Schema, presentations, values)
	if err != nil {
		return nil, nil, err
	}
	resolved := make(configurationcontract.ResolvedCandidates)
	if schemaHasDynamicCandidates(snapshot.Schema) {
		if s.dynamicCandidateResolver == nil {
			return nil, nil, bizerr.NewCode(CodePublicCatalogUnavailable)
		}
		var err error
		resolved, err = s.dynamicCandidateResolver.ResolvePublicPurchaseCandidates(
			ctx, snapshot.Target.VersionNo, snapshot.Schema, values,
		)
		if err != nil {
			return nil, nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
		}
	}
	evaluation, err := evaluateStructuredResolved(snapshot, values, resolved)
	return evaluation, resolved, err
}

// evaluateStructuredResolved applies existing declarations and commercial
// restrictions to one directory observation without additional owner calls.
func evaluateStructuredResolved(snapshot *publicStructuredConfigurationSnapshot, values []configurationcontract.ConfigurationValue, resolved configurationcontract.ResolvedCandidates) (*configurationcontract.Evaluation, error) {
	presentations, err := configurationcontract.DecodePresentations(snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, err
	}
	if err = configurationcontract.ValidatePresentations(snapshot.Schema, presentations); err != nil {
		return nil, err
	}
	values, err = configurationcontract.ApplyHiddenDefaults(snapshot.Schema, presentations, values)
	if err != nil {
		return nil, err
	}
	// Evaluate commercial restrictions after exact directory admission, while
	// retaining the provider declaration used by frozen evidence and digests.
	effective, err := configurationcontract.WithPresentationChoiceRules(snapshot.Schema, presentations)
	if err != nil {
		return nil, err
	}
	evaluation, err := configurationcontract.EvaluateResolved(
		effective, values, configurationcontract.SurfaceCustomer,
		authorizedDynamicMethods(snapshot.Schema), resolved,
	)
	if err == nil {
		evaluation.Issues = append(evaluation.Issues, configurationcontract.PresentationIssues(snapshot.Schema, presentations, evaluation.Values)...)
		evaluation.Issues = append(evaluation.Issues, missingPurchaseChoiceIssues(snapshot.Schema, presentations, evaluation)...)
		evaluation.Valid = len(evaluation.Issues) == 0
	}
	return evaluation, err
}

func schemaHasDynamicCandidates(schema *capmodel.ConfigurationSchemaDeclaration) bool {
	if schema == nil {
		return false
	}
	for _, field := range schema.Fields {
		if field != nil && field.DynamicCandidates != nil {
			return true
		}
	}
	return false
}

func authorizedDynamicMethods(schema *capmodel.ConfigurationSchemaDeclaration) []string {
	seen := make(map[string]struct{})
	methods := make([]string, 0)
	if schema == nil {
		return methods
	}
	for _, field := range schema.Fields {
		if field == nil || field.DynamicCandidates == nil {
			continue
		}
		method := field.DynamicCandidates.Method
		if _, exists := seen[method]; exists {
			continue
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	return methods
}

func (s *publicCatalogReaderImpl) decodePublicStructuredValues(
	snapshot *publicStructuredConfigurationSnapshot,
	input []PublicPurchaseConfigurationValue,
) ([]configurationcontract.ConfigurationValue, error) {
	if len(input) > maximumPublicStructuredConfigurationValues {
		return nil, gerror.New("public configuration value bound exceeded")
	}
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	fieldKeys := make(map[string]string)
	if snapshot.Schema != nil {
		for _, field := range snapshot.Schema.Fields {
			fieldToken, err := s.revision.encodeStructuredField(snapshot.Revision, field.Key)
			if err != nil {
				return nil, err
			}
			fields[fieldToken], fieldKeys[fieldToken] = field, field.Key
		}
	}
	result := make([]configurationcontract.ConfigurationValue, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, row := range input {
		field := fields[row.FieldToken]
		if field == nil || row.Type != field.Type {
			return nil, gerror.New("public configuration field token is invalid")
		}
		if _, duplicate := seen[row.FieldToken]; duplicate {
			return nil, gerror.New("public configuration field token is duplicated")
		}
		seen[row.FieldToken] = struct{}{}
		mapped := configurationcontract.ConfigurationValue{
			FieldKey: fieldKeys[row.FieldToken], TextValue: row.TextValue,
			IntegerTextValue: row.IntegerValue, DecimalValue: row.DecimalValue,
			BooleanValue: row.BooleanValue, DateValue: row.DateValue,
			DateTimeValue: row.DateTimeValue,
		}
		var err error
		mapped.OptionTokens, err = s.decodeOpaqueOptions(row.OptionTokens, snapshot.Revision, fieldKeys[row.FieldToken])
		if err != nil {
			return nil, err
		}
		mapped.TreeOptionTokens, err = s.decodeOpaqueOptions(row.TreeOptionTokens, snapshot.Revision, fieldKeys[row.FieldToken])
		if err != nil {
			return nil, err
		}
		mapped.AddressOptionTokens, err = s.decodeOpaqueOptions(row.AddressOptionTokens, snapshot.Revision, fieldKeys[row.FieldToken])
		if err != nil {
			return nil, err
		}
		mapped.FileTokens = row.FileTokens
		result = append(result, mapped)
	}
	presentations, err := configurationcontract.DecodePresentations(snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, err
	}
	return configurationcontract.ApplyHiddenDefaults(snapshot.Schema, presentations, result)
}

func (s *publicCatalogReaderImpl) decodeOpaqueOptions(tokens []string, revision, fieldKey string) ([]string, error) {
	if tokens == nil {
		return nil, nil
	}
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		value, err := s.revision.decodeStructuredOption(token, revision, fieldKey)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (s *publicCatalogReaderImpl) projectStructuredConfiguration(
	snapshot *publicStructuredConfigurationSnapshot,
	evaluation *configurationcontract.Evaluation,
	resolved configurationcontract.ResolvedCandidates,
) (*PublicPurchaseConfigurationView, error) {
	view := &PublicPurchaseConfigurationView{
		ServerLoginPolicy:     publicServerLoginPolicy(snapshot.Target.MerchantValues),
		Ready:                 evaluation.Valid,
		ConfigurationRevision: snapshot.Revision,
		Fields:                []PublicPurchaseConfigurationField{}, Values: []PublicPurchaseConfigurationValue{},
	}
	if snapshot.Schema == nil {
		return view, nil
	}
	presentations, err := configurationcontract.DecodePresentations(snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, err
	}
	roles, err := publicPurchasePresentationRoles(snapshot.Target, snapshot.Schema, presentations)
	if err != nil {
		return nil, err
	}
	orders := map[string]int{}
	for _, field := range snapshot.Schema.Fields {
		state := evaluation.States[field.Key]
		fieldToken, err := s.revision.encodeStructuredField(snapshot.Revision, field.Key)
		if err != nil {
			return nil, err
		}
		projected := PublicPurchaseConfigurationField{
			Presentation: roles[field.Key],
			FieldToken:   fieldToken, Type: field.Type,
			GroupZhCN: presentations[field.Key].GroupZhCN, GroupEnUS: presentations[field.Key].GroupEnUS, Unit: presentations[field.Key].Unit, Advanced: presentations[field.Key].Advanced, NumericLabels: presentations[field.Key].NumericLabels,
			DisplayControl: presentations[field.Key].Control, QuantityStep: presentations[field.Key].Step, NumericChoices: presentations[field.Key].NumericChoices,
			Label: publicLocalized(field.Label), Description: publicLocalized(field.Description),
			Visible: state.Visible && (presentations[field.Key].CustomerVisible == nil || *presentations[field.Key].CustomerVisible), Required: state.Required, Disabled: state.Disabled,
			Constraints: PublicPurchaseConfigurationConstraints{
				Format: field.TextFormat, Minimum: field.Minimum, Maximum: field.Maximum,
				MinimumLength: field.MinimumLength, MaximumLength: field.MaximumLength,
			},
		}
		if p := presentations[field.Key]; p.DescriptionZhCN != "" || p.DescriptionEnUS != "" {
			projected.Description = PublicLocalizedText{ZhCN: p.DescriptionZhCN, EnUS: p.DescriptionEnUS}
		}
		projected.Options, err = s.projectStructuredOptions(snapshot.Revision, field, state.CandidateValues, resolved[field.Key])
		if err != nil {
			return nil, err
		}
		if field.Type == capmodel.ConfigurationFieldFile {
			policy := configurationUploadPolicy(snapshot.Schema, field.UploadPolicy)
			if policy == nil {
				return nil, gerror.New("configuration upload policy is unavailable")
			}
			projected.UploadPolicy = &PublicPurchaseConfigurationUploadPolicy{
				MaximumFiles: policy.MaximumFiles, MaximumFileBytes: policy.MaximumFileBytes,
				MaximumTotalBytes:  policy.MaximumTotalBytes,
				AllowedMIMETypes:   append([]string(nil), policy.AllowedMIMETypes...),
				AllowedExtensions:  append([]string(nil), policy.AllowedExtensions...),
				SafetyScanRequired: policy.SafetyScanRequired,
			}
		}
		if presentation, exists := presentations[field.Key]; exists {
			orders[fieldToken] = presentation.SortOrder
		} else {
			orders[fieldToken] = len(view.Fields)
		}
		view.Fields = append(view.Fields, projected)
	}
	groups := map[string]int{}
	for _, field := range view.Fields {
		key := field.GroupZhCN + "\x00" + field.GroupEnUS
		rank, exists := groups[key]
		if !exists || orders[field.FieldToken] < rank {
			groups[key] = orders[field.FieldToken]
		}
	}
	sort.SliceStable(view.Fields, func(i, j int) bool {
		left, right := view.Fields[i], view.Fields[j]
		leftGroup, rightGroup := groups[left.GroupZhCN+"\x00"+left.GroupEnUS], groups[right.GroupZhCN+"\x00"+right.GroupEnUS]
		if leftGroup != rightGroup {
			return leftGroup < rightGroup
		}
		leftKey, rightKey := left.GroupZhCN+"\x00"+left.GroupEnUS, right.GroupZhCN+"\x00"+right.GroupEnUS
		if leftKey != rightKey {
			return leftKey < rightKey
		}
		return orders[left.FieldToken] < orders[right.FieldToken]
	})
	for _, value := range evaluation.Values {
		projected, err := s.projectStructuredValue(snapshot, value)
		if err != nil {
			return nil, err
		}
		view.Values = append(view.Values, projected)
	}
	return view, nil
}

func (s *publicCatalogReaderImpl) ResolvePurchaseFileTarget(
	ctx context.Context,
	publicSlug string,
	configurationRevision string,
	fieldToken string,
) (*purchaseconfigurationfile.Target, error) {
	snapshot, err := s.loadStructuredConfiguration(ctx, publicSlug, configurationRevision)
	if err != nil {
		return nil, err
	}
	for _, field := range snapshot.Schema.Fields {
		if field == nil || field.Type != capmodel.ConfigurationFieldFile {
			continue
		}
		expectedToken, tokenErr := s.revision.encodeStructuredField(snapshot.Revision, field.Key)
		if tokenErr != nil {
			return nil, tokenErr
		}
		if expectedToken != fieldToken {
			continue
		}
		policy := configurationUploadPolicy(snapshot.Schema, field.UploadPolicy)
		if policy == nil {
			return nil, gerror.New("purchase file policy is unavailable")
		}
		return &purchaseconfigurationfile.Target{
			ProductVersionNo:      snapshot.Target.VersionNo,
			ConfigurationRevision: snapshot.Revision,
			FieldKey:              field.Key,
			Policy:                policy,
		}, nil
	}
	return nil, gerror.New("purchase file field is unavailable")
}

func (s *publicCatalogReaderImpl) verifyStructuredFiles(
	ctx context.Context,
	snapshot *publicStructuredConfigurationSnapshot,
	values []configurationcontract.ConfigurationValue,
) error {
	inputs := make([]purchaseconfigurationfile.VerifyInput, 0)
	for _, value := range values {
		if value.FileTokens == nil {
			continue
		}
		field := configurationFieldByKey(snapshot.Schema, value.FieldKey)
		if field == nil || field.Type != capmodel.ConfigurationFieldFile {
			return gerror.New("purchase file field is invalid")
		}
		policy := configurationUploadPolicy(snapshot.Schema, field.UploadPolicy)
		if policy == nil {
			return gerror.New("purchase file policy is unavailable")
		}
		inputs = append(inputs, purchaseconfigurationfile.VerifyInput{
			Target: &purchaseconfigurationfile.Target{
				ProductVersionNo:      snapshot.Target.VersionNo,
				ConfigurationRevision: snapshot.Revision,
				FieldKey:              field.Key, Policy: policy,
			},
			FileTokens: value.FileTokens,
		})
	}
	if len(inputs) == 0 {
		return nil
	}
	if s.purchaseFiles == nil {
		return gerror.New("purchase file verification is unavailable")
	}
	_, err := s.purchaseFiles.VerifyFields(ctx, inputs)
	return err
}

func configurationUploadPolicy(schema *capmodel.ConfigurationSchemaDeclaration, policyID string) *capmodel.ConfigurationUploadPolicyDeclaration {
	if schema == nil {
		return nil
	}
	for _, policy := range schema.UploadPolicies {
		if policy != nil && policy.ID == policyID {
			return policy
		}
	}
	return nil
}

func (s *publicCatalogReaderImpl) projectStructuredOptions(
	revision string,
	field *capmodel.ConfigurationFieldDeclaration,
	allowed []string,
	dynamic []*capmodel.ConfigurationCandidate,
) ([]PublicPurchaseConfigurationOption, error) {
	candidates := field.StaticCandidates
	if field.DynamicCandidates != nil {
		candidates = dynamic
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		allowedSet[value] = struct{}{}
	}
	var attach func(string) ([]PublicPurchaseConfigurationOption, error)
	attach = func(parent string) ([]PublicPurchaseConfigurationOption, error) {
		items := make([]PublicPurchaseConfigurationOption, 0)
		for _, candidate := range candidates {
			if candidate.ParentValue != parent {
				continue
			}
			if _, visible := allowedSet[candidate.Value]; !visible {
				continue
			}
			token, err := s.revision.encodeStructuredOption(revision, field.Key, candidate.Value)
			if err != nil {
				return nil, err
			}
			children, err := attach(candidate.Value)
			if err != nil {
				return nil, err
			}
			items = append(items, PublicPurchaseConfigurationOption{
				OptionToken: token, Label: publicLocalized(candidate.Label), Children: children,
			})
		}
		return items, nil
	}
	return attach("")
}

func (s *publicCatalogReaderImpl) projectStructuredValue(
	snapshot *publicStructuredConfigurationSnapshot,
	value configurationcontract.ConfigurationValue,
) (PublicPurchaseConfigurationValue, error) {
	field := configurationFieldByKey(snapshot.Schema, value.FieldKey)
	if field == nil {
		return PublicPurchaseConfigurationValue{}, gerror.New("configuration value field is unavailable")
	}
	fieldToken, err := s.revision.encodeStructuredField(snapshot.Revision, value.FieldKey)
	if err != nil {
		return PublicPurchaseConfigurationValue{}, err
	}
	row := PublicPurchaseConfigurationValue{
		FieldToken: fieldToken, Type: field.Type,

		TextValue:    value.TextValue,
		DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue,
		DateValue: value.DateValue, DateTimeValue: value.DateTimeValue,
	}
	if value.IntegerValue != nil {
		text := fmt.Sprintf("%d", *value.IntegerValue)
		row.IntegerValue = &text
	}
	row.OptionTokens, err = s.projectOptionValues(snapshot.Revision, field, value.OptionTokens)
	if err != nil {
		return PublicPurchaseConfigurationValue{}, err
	}
	row.TreeOptionTokens, err = s.projectOptionValues(snapshot.Revision, field, value.TreeOptionTokens)
	if err != nil {
		return PublicPurchaseConfigurationValue{}, err
	}
	row.AddressOptionTokens, err = s.projectOptionValues(snapshot.Revision, field, value.AddressOptionTokens)
	if err != nil {
		return PublicPurchaseConfigurationValue{}, err
	}
	row.FileTokens = append([]string(nil), value.FileTokens...)
	return row, nil
}

func (s *publicCatalogReaderImpl) projectOptionValues(
	revision string,
	field *capmodel.ConfigurationFieldDeclaration,
	values []string,
) ([]string, error) {
	if values == nil {
		return nil, nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		token, err := s.revision.encodeStructuredOption(revision, field.Key, value)
		if err != nil {
			return nil, err
		}
		result = append(result, token)
	}
	return result, nil
}

func configurationFieldByKey(schema *capmodel.ConfigurationSchemaDeclaration, key string) *capmodel.ConfigurationFieldDeclaration {
	if schema == nil {
		return nil
	}
	for _, field := range schema.Fields {
		if field.Key == key {
			return field
		}
	}
	return nil
}

func publicLocalized(value *capmodel.LocalizedText) PublicLocalizedText {
	if value == nil {
		return PublicLocalizedText{}
	}
	return PublicLocalizedText{ZhCN: value.ZhCN, EnUS: value.EnUS}
}

func publicBillingTermLabel(term PublicQuoteTerm, resources PublicBillingLabelResources) (PublicLocalizedText, error) {
	if term.ChargeType == ChargeTypeOneTime {
		value := resources["charge.one_time"]
		if value.ZhCN == "" || value.EnUS == "" {
			return PublicLocalizedText{}, gerror.New("one-time billing label is unavailable")
		}
		return value, nil
	}
	unit := resources["period."+string(term.PeriodUnit)]
	if unit.ZhCN == "" || unit.EnUS == "" {
		return PublicLocalizedText{}, gerror.New("recurring billing period label is unavailable")
	}
	unitZh, unitEn := unit.ZhCN, unit.EnUS
	if term.PeriodCount != 1 {
		unitEn += "s"
	}
	return PublicLocalizedText{
		ZhCN: fmt.Sprintf("%d%s", term.PeriodCount, unitZh),
		EnUS: fmt.Sprintf("%d %s", term.PeriodCount, unitEn),
	}, nil
}

const publicStructuredConfigurationTargetSQL = `
SELECT product.product_no, product.current_version_no AS version_no,
       COALESCE(configuration.version, 0) AS configuration_version,
       COALESCE(configuration.merchant_value_snapshot,'{}'::jsonb)::text AS merchant_values,
       COALESCE(configuration.product_declaration_snapshot,'{}'::jsonb)::text AS product_declaration,
       COALESCE((SELECT binding.plugin_id FROM plugin_idc_core_catalog_product_version_fulfillment_binding binding WHERE binding.tenant_id=version.tenant_id AND binding.version_no=version.version_no LIMIT 1),'') AS binding_plugin_id,
       COALESCE(configuration.customer_presentation_snapshot,'{}'::jsonb)::text AS customer_presentation,
	   COALESCE(configuration.customer_declaration_snapshot, '{}'::jsonb)::text AS customer_declaration,
       COALESCE(configuration.declaration_digest, '') AS customer_declaration_digest,
       EXISTS (
           SELECT 1 FROM plugin_idc_core_catalog_product_version_fulfillment_binding binding
           WHERE binding.tenant_id = version.tenant_id AND binding.version_no = version.version_no
       ) AS has_automation_binding
FROM plugin_idc_core_catalog_product product
INNER JOIN plugin_idc_core_catalog_category category
    ON category.tenant_id = product.tenant_id
   AND category.category_no = product.category_no
   AND category.deleted_at IS NULL AND category.status_code = 'active'
INNER JOIN plugin_idc_core_catalog_product_version version
    ON version.tenant_id = product.tenant_id
   AND version.product_no = product.product_no
   AND version.version_no = product.current_version_no
   AND version.status_code = 'published'
LEFT JOIN plugin_idc_core_catalog_product_version_configuration configuration
    ON configuration.tenant_id = version.tenant_id
   AND configuration.version_no = version.version_no
WHERE product.tenant_id = 0 AND product.deleted_at IS NULL
  AND product.status_code = 'active' AND product.public_slug = ?
LIMIT 1`

const publicStructuredConfigurationStillCurrentSQL = `
SELECT EXISTS (
    SELECT 1
    FROM plugin_idc_core_catalog_product product
    INNER JOIN plugin_idc_core_catalog_category category
        ON category.tenant_id = product.tenant_id
       AND category.category_no = product.category_no
       AND category.deleted_at IS NULL AND category.status_code = 'active'
    INNER JOIN plugin_idc_core_catalog_product_version version
        ON version.tenant_id = product.tenant_id
       AND version.product_no = product.product_no
       AND version.version_no = product.current_version_no
       AND version.status_code = 'published'
    LEFT JOIN plugin_idc_core_catalog_product_version_configuration configuration
        ON configuration.tenant_id = version.tenant_id
       AND configuration.version_no = version.version_no
    WHERE product.tenant_id = 0 AND product.deleted_at IS NULL
      AND product.status_code = 'active' AND version.version_no = ?
      AND COALESCE(configuration.version, 0) = ?
      AND COALESCE(configuration.declaration_digest, '') = ?
)`

var _ PublicStructuredConfigurationReader = (*publicCatalogReaderImpl)(nil)
