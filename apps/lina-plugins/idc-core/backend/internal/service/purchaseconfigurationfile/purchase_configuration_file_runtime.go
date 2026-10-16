package purchaseconfigurationfile

import (
	"context"
	"io"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
)

var runtimeSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ConsumedFileScope is trusted fulfillment evidence. None of these values may
// come from a provider payload.
type ConsumedFileScope struct {
	IntentNo, OrderNo, OrderItemNo, ConfigurationSnapshotNo string
	PluginID, ReleaseVersion, ContractVersion               string
	ReleaseDigest, DeclarationDigest, Method                string
	InterfaceInstanceNo                                     string
	InterfaceConfigurationRevision                          int64
}

// ConsumedFile is owner-private immutable evidence. ObjectRef must never leave
// idc-core and is intentionally absent from supplierinterfacecap responses.
type ConsumedFile struct {
	Sequence  int
	FileName  string
	MediaType string
	SizeBytes int64
	SHA256Hex string
	ObjectRef string
}

type OpenConsumedFile struct {
	File ConsumedFile
	Body io.ReadCloser
}

type ConsumedFileReader interface {
	ListConsumedField(context.Context, ConsumedFileScope, string) ([]ConsumedFile, error)
	OpenConsumedFieldFile(context.Context, ConsumedFileScope, string, int) (*OpenConsumedFile, error)
	// VisitConsumedField reads metadata once and streams each verified private
	// object to a trusted in-process consumer without exposing storage paths.
	VisitConsumedField(context.Context, ConsumedFileScope, string, func(ConsumedFile, io.Reader) error) error
}

func (r *consumedFileReader) VisitConsumedField(ctx context.Context, scope ConsumedFileScope, fieldKey string, consume func(ConsumedFile, io.Reader) error) error {
	if consume == nil {
		return gerror.New("consumed file consumer is unavailable")
	}
	files, err := r.ListConsumedField(ctx, scope, fieldKey)
	if err != nil {
		return err
	}
	for _, file := range files {
		object, err := r.storage.Get(ctx, storagecap.GetInput{Path: file.ObjectRef})
		if err != nil {
			return err
		}
		if object == nil || object.Body == nil {
			return gerror.New("consumed file content is unavailable")
		}
		valid := object.Found && object.Object != nil && object.Object.Path == file.ObjectRef && object.Object.Size == file.SizeBytes && object.Object.ContentType == file.MediaType && object.Object.Visibility == storagecap.VisibilityPrivate
		if valid {
			// The consumer receives metadata only, never the private storage key.
			file.ObjectRef = ""
			err = consume(file, object.Body)
		} else {
			err = gerror.New("consumed file content evidence mismatch")
		}
		closeErr := object.Body.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

type consumedFileReader struct {
	database gdb.DB
	storage  storagecap.Service
}

func NewConsumedFileReader(database gdb.DB, storage storagecap.Service) (ConsumedFileReader, error) {
	if database == nil || storage == nil {
		return nil, gerror.New("consumed purchase configuration file reader is unavailable")
	}
	return &consumedFileReader{database: database, storage: storage}, nil
}

func (r *consumedFileReader) ListConsumedField(ctx context.Context, scope ConsumedFileScope, fieldKey string) ([]ConsumedFile, error) {
	if r == nil || r.database == nil || !validRuntimeScope(scope) || !validRuntimeFieldKey(fieldKey) {
		return nil, gerror.New("consumed purchase configuration file evidence is unavailable")
	}
	rows := make([]ConsumedFile, 0)
	err := r.database.GetScan(ctx, &rows, `SELECT file.file_sequence AS sequence,upload.file_name,upload.media_type,
upload.actual_size_bytes AS size_bytes,upload.sha256_hex,upload.object_ref
FROM plugin_idc_core_commerce_structured_configuration_snapshot snapshot
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id=snapshot.tenant_id AND intent.intent_no=? AND intent.order_no=snapshot.order_no
 AND intent.order_item_no=snapshot.order_item_no AND intent.configuration_snapshot_no=snapshot.snapshot_no
 AND intent.plugin_id=? AND intent.plugin_release_version=? AND intent.contract_version=?
 AND intent.release_digest=? AND intent.declaration_digest=? AND intent.interface_instance_no=?
 AND intent.interface_configuration_revision=? AND ? IN(intent.provision_method,intent.query_method)
JOIN plugin_idc_core_commerce_structured_configuration_file_snapshot file
  ON file.tenant_id=snapshot.tenant_id AND file.snapshot_no=snapshot.snapshot_no
JOIN plugin_idc_core_purchase_configuration_upload upload
  ON upload.tenant_id=file.tenant_id AND upload.upload_no=file.upload_no
WHERE snapshot.tenant_id=0 AND snapshot.snapshot_no=? AND snapshot.order_no=? AND snapshot.order_item_no=?
  AND file.field_key=? AND upload.status_code='consumed' AND upload.bound_owner_type='order'
  AND upload.bound_owner_no=snapshot.order_no AND upload.customer_no=snapshot.customer_no
  AND upload.uploader_user_id=snapshot.uploader_user_id AND upload.product_version_no=snapshot.version_no
  AND upload.configuration_revision=snapshot.configuration_revision AND upload.field_key=file.field_key
  AND upload.scan_status IN('clean','not_required')
ORDER BY file.file_sequence LIMIT 101`, scope.IntentNo, scope.PluginID, scope.ReleaseVersion, scope.ContractVersion,
		scope.ReleaseDigest, scope.DeclarationDigest, scope.InterfaceInstanceNo, scope.InterfaceConfigurationRevision, scope.Method,
		scope.ConfigurationSnapshotNo, scope.OrderNo, scope.OrderItemNo, fieldKey)
	if err != nil || len(rows) > 100 {
		return nil, gerror.New("consumed purchase configuration file evidence is unavailable")
	}
	for index, row := range rows {
		if row.Sequence != index+1 || row.FileName == "" || row.MediaType == "" || row.SizeBytes < 1 ||
			!runtimeSHA256Pattern.MatchString(row.SHA256Hex) || strings.TrimSpace(row.ObjectRef) == "" {
			return nil, gerror.New("consumed purchase configuration file evidence is unavailable")
		}
	}
	return rows, nil
}

func (r *consumedFileReader) OpenConsumedFieldFile(ctx context.Context, scope ConsumedFileScope, fieldKey string, sequence int) (*OpenConsumedFile, error) {
	files, err := r.ListConsumedField(ctx, scope, fieldKey)
	if err != nil || sequence < 1 || sequence > len(files) {
		return nil, gerror.New("consumed purchase configuration file is unavailable")
	}
	file := files[sequence-1]
	object, err := r.storage.Get(ctx, storagecap.GetInput{Path: file.ObjectRef})
	if err != nil || object == nil || !object.Found || object.Object == nil || object.Body == nil ||
		object.Object.Path != file.ObjectRef || object.Object.Size != file.SizeBytes || object.Object.ContentType != file.MediaType || object.Object.Visibility != storagecap.VisibilityPrivate {
		if object != nil && object.Body != nil {
			_ = object.Body.Close()
		}
		return nil, gerror.New("consumed purchase configuration file content is unavailable")
	}
	return &OpenConsumedFile{File: file, Body: object.Body}, nil
}

func validRuntimeScope(scope ConsumedFileScope) bool {
	return validBoundOwnerNo(scope.IntentNo) && validBoundOwnerNo(scope.OrderNo) && validBoundOwnerNo(scope.OrderItemNo) && validBoundOwnerNo(scope.ConfigurationSnapshotNo) &&
		validRuntimePluginID(scope.PluginID) && scope.ReleaseVersion != "" && len(scope.ReleaseVersion) <= 32 && scope.ContractVersion != "" && len(scope.ContractVersion) <= 32 &&
		runtimeSHA256Pattern.MatchString(scope.ReleaseDigest) && runtimeSHA256Pattern.MatchString(scope.DeclarationDigest) && validRuntimeMethod(scope.Method) &&
		validBoundOwnerNo(scope.InterfaceInstanceNo) && scope.InterfaceConfigurationRevision > 0
}

func validRuntimePluginID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, part := range strings.Split(value, "-") {
		if part == "" {
			return false
		}
		for _, character := range part {
			if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
				continue
			}
			return false
		}
	}
	return true
}

func validRuntimeMethod(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return value[0] >= 'a' && value[0] <= 'z'
}

func validRuntimeFieldKey(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}
