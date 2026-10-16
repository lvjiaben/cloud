package commerce

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const maximumStructuredSnapshotsPerOrder = 100

type FreezeQuoteItemConfigurationInput struct {
	QuoteItemNo string
	Purchase    catalog.PublicPurchaseConfigurationInput
}

type FrozenQuoteItemConfiguration struct {
	SnapshotNo            string
	ConfigurationFrozen   bool
	ConfigurationRevision string
	CreatedAt             time.Time
}

type structuredConfigurationService struct {
	database gdb.DB
	freezer  catalog.PurchaseConfigurationFreezer
	files    purchaseconfigurationfile.OrderFileConsumer
}

type structuredSnapshotQuoteItem struct {
	QuoteNo              string    `orm:"quote_no"`
	QuoteItemNo          string    `orm:"quote_item_no"`
	CustomerNo           string    `orm:"customer_no"`
	ProductNo            string    `orm:"product_no"`
	VersionNo            string    `orm:"version_no"`
	PriceSelectionDigest string    `orm:"price_selection_digest"`
	CustomerValueDigest  string    `orm:"customer_value_digest"`
	ExpiresAt            time.Time `orm:"expires_at"`
}

type structuredFileRow struct {
	TenantID             int    `orm:"tenant_id"`
	SnapshotNo           string `orm:"snapshot_no"`
	FieldSequence        int    `orm:"field_sequence"`
	FileSequence         int    `orm:"file_sequence"`
	FieldKey             string `orm:"field_key"`
	UploadNo             string `orm:"upload_no"`
	UploadPolicySnapshot string `orm:"upload_policy_snapshot"`
}

type structuredCheckoutSnapshotRow struct {
	SnapshotNo            string `orm:"snapshot_no"`
	CustomerNo            string `orm:"customer_no"`
	UploaderUserID        int    `orm:"uploader_user_id"`
	QuoteItemNo           string `orm:"quote_item_no"`
	VersionNo             string `orm:"version_no"`
	ConfigurationRevision string `orm:"configuration_revision"`
}

type structuredCheckoutFileRow struct {
	SnapshotNo           string `orm:"snapshot_no"`
	FieldSequence        int    `orm:"field_sequence"`
	FileSequence         int    `orm:"file_sequence"`
	FieldKey             string `orm:"field_key"`
	UploadNo             string `orm:"upload_no"`
	UploadPolicySnapshot string `orm:"upload_policy_snapshot"`
}

type structuredRequiredQuoteItem struct {
	QuoteItemNo string `orm:"quote_item_no"`
}

type structuredSnapshotDigestInput struct {
	CustomerNo, QuoteNo, QuoteItemNo, ProductNo, VersionNo, ConfigurationRevision                string
	UploaderUserID                                                                               int
	InterfaceConfigurationRevision                                                               int64
	InterfaceInstanceNo, CustomerDeclarationDigest, CandidateSnapshotDigest, CustomerValueDigest string
}

func BindStructuredConfigurationSnapshots(service Service, freezer catalog.PurchaseConfigurationFreezer, files purchaseconfigurationfile.OrderFileConsumer) error {
	impl, ok := service.(*serviceImpl)
	if !ok || impl == nil || isNilStructuredDependency(freezer) || isNilStructuredDependency(files) || impl.structuredConfiguration != nil {
		return errors.New("commerce structured configuration requires one freezer and file service")
	}
	impl.structuredConfiguration = &structuredConfigurationService{database: impl.database, freezer: freezer, files: files}
	return nil
}

func isNilStructuredDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return (reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface) && reflected.IsNil()
}

func (s *serviceImpl) FreezeAccountQuoteItemConfiguration(ctx context.Context, input FreezeQuoteItemConfigurationInput) (*FrozenQuoteItemConfiguration, error) {
	audience, err := s.currentCheckoutAudience(ctx)
	if err != nil {
		return nil, err
	}
	return s.freezeQuoteItemConfiguration(ctx, audience.CustomerNo, audience.UserID, input)
}

func (s *serviceImpl) freezeQuoteItemConfiguration(
	ctx context.Context,
	customerNo string,
	uploaderUserID int,
	input FreezeQuoteItemConfigurationInput,
) (*FrozenQuoteItemConfiguration, error) {
	if s.structuredConfiguration == nil || strings.TrimSpace(input.QuoteItemNo) != input.QuoteItemNo || input.QuoteItemNo == "" || len(input.QuoteItemNo) > 40 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	quoteItem, err := s.structuredConfiguration.loadQuoteItem(ctx, customerNo, input.QuoteItemNo, false)
	if err != nil {
		return nil, err
	}
	frozen, err := s.structuredConfiguration.freezer.FreezePurchaseConfiguration(ctx, input.Purchase)
	if err != nil {
		return nil, err
	}
	if frozen.ProductNo != quoteItem.ProductNo || frozen.VersionNo != quoteItem.VersionNo ||
		!constantTimeDigestEqual(frozen.PriceSelectionDigest, quoteItem.PriceSelectionDigest) ||
		!constantTimeDigestEqual(frozen.CustomerValueDigest, quoteItem.CustomerValueDigest) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	var clock struct {
		Now time.Time `orm:"now"`
	}
	if err = s.database.GetScan(ctx, &clock, "SELECT CURRENT_TIMESTAMP AS now"); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	now := clock.Now
	if !quoteItem.ExpiresAt.After(now) {
		return nil, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	snapshotNo, err := newOpaqueNumber(s.random, "SCF_")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	digest, err := quoteContentDigest(&structuredSnapshotDigestInput{
		customerNo, quoteItem.QuoteNo, quoteItem.QuoteItemNo, frozen.ProductNo, frozen.VersionNo,
		frozen.ConfigurationRevision, uploaderUserID, frozen.InterfaceConfigurationRevision, frozen.InterfaceInstanceNo,
		frozen.CustomerDeclarationDigest, frozen.CandidateSnapshotDigest, frozen.CustomerValueDigest})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	fileRows, err := structuredSnapshotFileRows(snapshotNo, frozen.FileFields)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		locked, loadErr := s.structuredConfiguration.loadQuoteItem(ctx, customerNo, input.QuoteItemNo, true)
		if loadErr != nil {
			return loadErr
		}
		if locked.ProductNo != frozen.ProductNo || locked.VersionNo != frozen.VersionNo || !locked.ExpiresAt.After(now) ||
			!constantTimeDigestEqual(frozen.PriceSelectionDigest, locked.PriceSelectionDigest) ||
			!constantTimeDigestEqual(frozen.CustomerValueDigest, locked.CustomerValueDigest) {
			return bizerr.NewCode(CodeConflict)
		}
		var existing struct{ SnapshotNo, ContentDigest string }
		loadErr = tx.GetScan(&existing, `SELECT snapshot_no, content_digest FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE tenant_id=0 AND quote_item_no=?`, input.QuoteItemNo)
		if loadErr == nil && existing.SnapshotNo != "" {
			if existing.ContentDigest != digest {
				return bizerr.NewCode(CodeConflict)
			}
			snapshotNo = existing.SnapshotNo
			return nil
		}
		if loadErr != nil && !errors.Is(loadErr, sql.ErrNoRows) {
			return gerror.Wrap(loadErr, "load frozen quote item configuration")
		}
		_, loadErr = tx.Exec(`INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot
(tenant_id,snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest,created_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,CAST(? AS jsonb),CAST(? AS jsonb),?,?)`, snapshotNo, customerNo, uploaderUserID,
			quoteItem.QuoteNo, quoteItem.QuoteItemNo, frozen.ProductNo, frozen.VersionNo, frozen.ConfigurationRevision,
			frozen.InterfaceInstanceNo, frozen.InterfaceConfigurationRevision, frozen.CustomerDeclarationDigest,
			frozen.CandidateSnapshotDigest, frozen.CustomerValueDigest, string(frozen.CandidateSnapshotJSON),
			string(frozen.TypedValueSnapshotJSON), digest, now)
		if loadErr != nil {
			return gerror.Wrap(loadErr, "insert frozen quote item configuration")
		}
		for index := range fileRows {
			fileRows[index].SnapshotNo = snapshotNo
		}
		if len(fileRows) > 0 {
			_, loadErr = tx.Model("plugin_idc_core_commerce_structured_configuration_file_snapshot").Ctx(ctx).Data(fileRows).Insert()
		}
		return loadErr
	})
	if err != nil {
		return nil, err
	}
	return &FrozenQuoteItemConfiguration{
		SnapshotNo: snapshotNo, ConfigurationFrozen: true,
		ConfigurationRevision: frozen.ConfigurationRevision, CreatedAt: now,
	}, nil
}

func (s *structuredConfigurationService) loadQuoteItem(ctx context.Context, customerNo, quoteItemNo string, lock bool) (*structuredSnapshotQuoteItem, error) {
	query := `SELECT quote.quote_no, item.quote_item_no, quote.customer_no, item.product_no, item.version_no,
configuration.selection_digest AS price_selection_digest,configuration.customer_value_digest,quote.expires_at
FROM plugin_idc_core_commerce_quote_item_snapshot item
JOIN plugin_idc_core_commerce_quote_snapshot quote ON quote.quote_no=item.quote_no AND quote.tenant_id=item.tenant_id
JOIN plugin_idc_core_commerce_configuration_snapshot configuration
  ON configuration.configuration_snapshot_no=item.configuration_snapshot_no
 AND configuration.version_no=item.version_no AND configuration.tenant_id=item.tenant_id
WHERE item.tenant_id=0 AND item.quote_item_no=? AND quote.customer_no=?`
	if lock {
		query += " FOR UPDATE OF item, quote"
	}
	var row structuredSnapshotQuoteItem
	if err := s.database.GetScan(ctx, &row, query, quoteItemNo, customerNo); err != nil || row.QuoteItemNo == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &row, nil
}

func constantTimeDigestEqual(left, right string) bool {
	return len(left) == 64 && len(right) == 64 && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func structuredSnapshotFileRows(snapshotNo string, inputs []purchaseconfigurationfile.VerifyInput) ([]structuredFileRow, error) {
	if len(inputs) > 128 {
		return nil, errors.New("structured file field bound exceeded")
	}
	rows := make([]structuredFileRow, 0)
	for fieldIndex, input := range inputs {
		if input.Target == nil || input.Target.Policy == nil {
			return nil, errors.New("structured file target is unavailable")
		}
		policy, err := json.Marshal(input.Target.Policy)
		if err != nil {
			return nil, err
		}
		for fileIndex, token := range input.FileTokens {
			rows = append(rows, structuredFileRow{TenantID: 0, SnapshotNo: snapshotNo, FieldSequence: fieldIndex + 1,
				FileSequence: fileIndex + 1, FieldKey: input.Target.FieldKey, UploadNo: token, UploadPolicySnapshot: string(policy)})
			if len(rows) > 12800 {
				return nil, errors.New("structured file token bound exceeded")
			}
		}
	}
	return rows, nil
}

func decodeStructuredPolicy(raw string) (*capmodel.ConfigurationUploadPolicyDeclaration, error) {
	var policy capmodel.ConfigurationUploadPolicyDeclaration
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

func (s *structuredConfigurationService) bindCheckout(ctx context.Context, tx gdb.TX, authority checkoutAuthority, quoteItems []entity.CommerceQuoteItemSnapshot, rows checkoutRows) error {
	if len(quoteItems) == 0 || len(quoteItems) > maximumStructuredSnapshotsPerOrder || len(rows.items) != len(quoteItems) {
		return bizerr.NewCode(CodeConflict)
	}
	quoteItemNos := make([]string, 0, len(quoteItems))
	orderItemByQuote := make(map[string]string, len(quoteItems))
	for index, item := range quoteItems {
		quoteItemNos = append(quoteItemNos, item.QuoteItemNo)
		orderItemByQuote[item.QuoteItemNo] = fmt.Sprint(rows.items[index].OrderItemNo)
	}
	var required []structuredRequiredQuoteItem
	if err := tx.GetScan(&required, `SELECT item.quote_item_no
FROM plugin_idc_core_commerce_quote_item_snapshot item
JOIN plugin_idc_core_catalog_product_version_configuration configuration
  ON configuration.tenant_id=item.tenant_id AND configuration.version_no=item.version_no
WHERE item.tenant_id=0 AND item.quote_item_no IN(?) ORDER BY item.quote_item_no`, quoteItemNos); err != nil {
		return gerror.Wrap(err, "load structured checkout requirements")
	}
	if len(required) == 0 {
		return nil
	}
	var snapshots []structuredCheckoutSnapshotRow
	if err := tx.GetScan(&snapshots, `SELECT snapshot_no,customer_no,uploader_user_id,quote_item_no,version_no,configuration_revision
FROM plugin_idc_core_commerce_structured_configuration_snapshot
WHERE tenant_id=0 AND quote_item_no IN(?) AND order_no IS NULL
ORDER BY quote_item_no FOR UPDATE`, quoteItemNos); err != nil {
		return gerror.Wrap(err, "lock structured checkout snapshots")
	}
	if len(snapshots) != len(required) {
		if authority.assistedRequestNo != nil {
			return bizerr.NewCode(CodeAssistedConfigurationMissing)
		}
		return gerror.Wrap(bizerr.NewCode(CodeQuoteNotAvailable), "structured snapshot count does not match required quote items")
	}
	configurationOwner := authority.userID
	if authority.assistedRequestNo != nil {
		var owner struct{ RequestedBy int }
		if err := tx.GetScan(&owner, `SELECT requested_by FROM plugin_idc_core_commerce_assisted_quote_request WHERE tenant_id=0 AND request_no=? AND customer_no=? AND quote_no=? AND status_code=?`, *authority.assistedRequestNo, authority.customerNo, quoteItems[0].QuoteNo, string(assistedQuoteStatusDirectReady)); err != nil || owner.RequestedBy <= 0 {
			return bizerr.NewCode(CodeNotAccessible)
		}
		configurationOwner = owner.RequestedBy
	}
	byQuote := make(map[string]structuredCheckoutSnapshotRow, len(snapshots))
	snapshotNos := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.CustomerNo != authority.customerNo || snapshot.UploaderUserID != configurationOwner || orderItemByQuote[snapshot.QuoteItemNo] == "" {
			return bizerr.NewCode(CodeNotAccessible)
		}
		byQuote[snapshot.QuoteItemNo] = snapshot
		snapshotNos = append(snapshotNos, snapshot.SnapshotNo)
	}
	for _, requiredItem := range required {
		if byQuote[requiredItem.QuoteItemNo].SnapshotNo == "" {
			return gerror.Wrap(bizerr.NewCode(CodeQuoteNotAvailable), "structured snapshot is missing for a required quote item")
		}
	}
	var files []structuredCheckoutFileRow
	if err := tx.GetScan(&files, `SELECT snapshot_no,field_sequence,file_sequence,field_key,upload_no,upload_policy_snapshot
FROM plugin_idc_core_commerce_structured_configuration_file_snapshot
WHERE tenant_id=0 AND snapshot_no IN(?) ORDER BY snapshot_no,field_sequence,file_sequence`, snapshotNos); err != nil {
		return gerror.Wrap(err, "load structured checkout file snapshots")
	}
	inputs, err := structuredCheckoutFileInputs(snapshots, files)
	if err != nil {
		return bizerr.NewCode(CodeConflict)
	}
	if len(inputs) > 0 {
		if _, err = s.files.ConsumeForOrderTransaction(ctx, tx, rows.orderNo, inputs); err != nil {
			return err
		}
	}
	arguments := make([]any, 0, 1+len(snapshots)*3+2)
	arguments = append(arguments, rows.orderNo)
	values := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		values = append(values, "(?,?,?)")
		arguments = append(arguments, snapshot.SnapshotNo, orderItemByQuote[snapshot.QuoteItemNo], snapshot.QuoteItemNo)
	}
	arguments = append(arguments, authority.customerNo, configurationOwner)
	result, err := tx.Exec(`UPDATE plugin_idc_core_commerce_structured_configuration_snapshot AS snapshot
SET order_no=?, order_item_no=source.order_item_no, bound_at=CURRENT_TIMESTAMP
FROM (VALUES `+strings.Join(values, ",")+`) AS source(snapshot_no,order_item_no,quote_item_no)
WHERE snapshot.tenant_id=0 AND snapshot.snapshot_no=source.snapshot_no AND snapshot.quote_item_no=source.quote_item_no
  AND snapshot.order_no IS NULL AND snapshot.customer_no=? AND snapshot.uploader_user_id=?`, arguments...)
	if err != nil {
		return gerror.Wrap(err, "bind structured checkout snapshots")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != int64(len(snapshots)) {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func structuredCheckoutFileInputs(snapshots []structuredCheckoutSnapshotRow, rows []structuredCheckoutFileRow) ([]purchaseconfigurationfile.VerifyInput, error) {
	if len(rows) > 12800 {
		return nil, errors.New("structured checkout file bound exceeded")
	}
	bySnapshot := make(map[string]structuredCheckoutSnapshotRow, len(snapshots))
	for _, snapshot := range snapshots {
		bySnapshot[snapshot.SnapshotNo] = snapshot
	}
	result := make([]purchaseconfigurationfile.VerifyInput, 0)
	var current *purchaseconfigurationfile.VerifyInput
	lastSnapshot := ""
	lastField := 0
	for _, row := range rows {
		snapshot := bySnapshot[row.SnapshotNo]
		if snapshot.SnapshotNo == "" {
			return nil, errors.New("structured checkout file snapshot is orphaned")
		}
		if row.SnapshotNo != lastSnapshot || row.FieldSequence != lastField {
			policy, err := decodeStructuredPolicy(row.UploadPolicySnapshot)
			if err != nil {
				return nil, err
			}
			result = append(result, purchaseconfigurationfile.VerifyInput{Target: &purchaseconfigurationfile.Target{
				ProductVersionNo: snapshot.VersionNo, ConfigurationRevision: snapshot.ConfigurationRevision,
				FieldKey: row.FieldKey, Policy: policy,
			}})
			current = &result[len(result)-1]
			lastSnapshot, lastField = row.SnapshotNo, row.FieldSequence
		}
		current.FileTokens = append(current.FileTokens, row.UploadNo)
	}
	return result, nil
}
