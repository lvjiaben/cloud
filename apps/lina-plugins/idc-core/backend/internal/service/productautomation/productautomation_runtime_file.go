package productautomation

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const (
	maximumRuntimeFileFields = 128
	maximumRuntimeFileFiles  = maximumRuntimeFileFields * 100
	// Reading 10 GiB can require one partial final chunk for every governed
	// file, in addition to one metadata call for every governed field.
	maximumRuntimeFileCalls = 10240 + maximumRuntimeFileFiles + maximumRuntimeFileFields
	maximumRuntimeFileBytes = int64(10 * 1024 * 1024 * 1024)
)

type supplierRuntimeFileSession struct {
	mu     sync.Mutex
	file   purchaseconfigurationfile.ConsumedFile
	body   io.ReadCloser
	offset int64
	closed bool
}

type supplierRuntimeFileStore struct {
	mu       sync.Mutex
	sessions map[string]*supplierRuntimeFileSession
}

func newSupplierRuntimeFileStore() *supplierRuntimeFileStore {
	return &supplierRuntimeFileStore{sessions: make(map[string]*supplierRuntimeFileSession)}
}

func (s *serviceImpl) releaseRuntimeContext(contextID string) {
	if s == nil || s.runtimeContexts == nil || s.runtimeFiles == nil || contextID == "" {
		return
	}
	s.runtimeContexts.mu.Lock()
	delete(s.runtimeContexts.items, contextID)
	s.runtimeFiles.mu.Lock()
	prefix := contextID + "\x00"
	closing := make([]*supplierRuntimeFileSession, 0)
	for key, session := range s.runtimeFiles.sessions {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(s.runtimeFiles.sessions, key)
			closing = append(closing, session)
		}
	}
	s.runtimeFiles.mu.Unlock()
	s.runtimeContexts.mu.Unlock()
	for _, session := range closing {
		session.mu.Lock()
		if !session.closed {
			_ = session.body.Close()
			session.closed = true
		}
		session.mu.Unlock()
	}
}

func (s *serviceImpl) attachRuntimeFile(contextID, callerPluginID, key string, candidate *supplierRuntimeFileSession) (*supplierRuntimeFileSession, bool) {
	if s == nil || candidate == nil || candidate.body == nil {
		return nil, false
	}
	s.runtimeContexts.mu.Lock()
	defer s.runtimeContexts.mu.Unlock()
	item, ok := s.runtimeContexts.items[contextID]
	if !ok || !s.runtimeContexts.now().Before(item.expiresAt) || item.callerPluginID != callerPluginID {
		return nil, false
	}
	s.runtimeFiles.mu.Lock()
	defer s.runtimeFiles.mu.Unlock()
	if current := s.runtimeFiles.sessions[key]; current != nil {
		return current, true
	}
	s.runtimeFiles.sessions[key] = candidate
	return candidate, true
}

func (s *supplierRuntimeContextStore) reserveFileCall(contextID, callerPluginID, fieldKey string, sequence int, requestedBytes int64) (supplierRuntimeContext, bool) {
	if s == nil || requestedBytes < 0 {
		return supplierRuntimeContext{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[contextID]
	if !ok || !s.now().Before(item.expiresAt) || item.callerPluginID != callerPluginID ||
		item.point != capmodel.BusinessExtensionProductFulfillment ||
		item.intentNo == "" || item.orderNo == "" || item.orderItemNo == "" || item.configurationSnapshotNo == "" ||
		item.fileReadCalls >= maximumRuntimeFileCalls || item.fileReadBytes > maximumRuntimeFileBytes-requestedBytes ||
		!containsString(item.authorizedMethods, item.method) {
		if ok && !s.now().Before(item.expiresAt) {
			delete(s.items, contextID)
		}
		return supplierRuntimeContext{}, false
	}
	item.fileReadCalls++
	item.fileReadBytes += requestedBytes
	if validRuntimeFieldKey(fieldKey) {
		_, fieldExists := item.fileReadFields[fieldKey]
		newFileKey := ""
		fileExists := true
		if sequence >= 1 && sequence <= 100 {
			newFileKey = fieldKey + "\x00" + strconv.Itoa(sequence)
			_, fileExists = item.fileReadFiles[newFileKey]
		}
		if !fieldExists && len(item.fileReadFields) >= maximumRuntimeFileFields ||
			!fileExists && len(item.fileReadFiles) >= maximumRuntimeFileFiles {
			s.items[contextID] = item
			return supplierRuntimeContext{}, false
		}
		if !fieldExists {
			item.fileReadFields[fieldKey] = struct{}{}
		}
		if !fileExists {
			item.fileReadFiles[newFileKey] = struct{}{}
		}
	}
	s.items[contextID] = item
	return item, true
}

func (s *serviceImpl) runtimeFileList(ctx context.Context, callerPluginID, contextID, fieldKey string) (*supplierinterfacecap.ConfigurationFileList, error) {
	runtimeContext, ok := s.runtimeContexts.reserveFileCall(contextID, callerPluginID, fieldKey, 0, 0)
	if !ok || s.consumedFiles == nil || !validRuntimeFieldKey(fieldKey) {
		return nil, gerror.New("supplier runtime file evidence is unavailable")
	}
	files, err := s.consumedFiles.ListConsumedField(ctx, runtimeFileScope(runtimeContext), fieldKey)
	if err != nil {
		return nil, gerror.New("supplier runtime file evidence is unavailable")
	}
	result := &supplierinterfacecap.ConfigurationFileList{Files: make([]supplierinterfacecap.ConfigurationFile, len(files))}
	for index, file := range files {
		result.Files[index] = supplierinterfacecap.ConfigurationFile{Sequence: file.Sequence, FileName: file.FileName, MediaType: file.MediaType, SizeBytes: file.SizeBytes, SHA256Hex: file.SHA256Hex}
	}
	if supplierinterfacecap.ValidateConfigurationFileList(result) != nil {
		return nil, gerror.New("supplier runtime file evidence is unavailable")
	}
	return result, nil
}

func (s *serviceImpl) runtimeFileRead(ctx context.Context, callerPluginID, contextID string, request supplierinterfacecap.ConfigurationFileReadRequest) (*supplierinterfacecap.ConfigurationFileChunk, error) {
	requestErr := supplierinterfacecap.ValidateConfigurationFileReadRequest(request)
	requestedBytes := int64(0)
	if requestErr == nil {
		requestedBytes = int64(request.MaximumBytes)
	}
	runtimeContext, ok := s.runtimeContexts.reserveFileCall(contextID, callerPluginID, request.FieldKey, request.Sequence, requestedBytes)
	if !ok || s.consumedFiles == nil || s.runtimeFiles == nil {
		return nil, gerror.New("supplier runtime file content is unavailable")
	}
	if requestErr != nil {
		return nil, gerror.New("supplier runtime file request is unavailable")
	}
	key := contextID + "\x00" + request.FieldKey + "\x00" + strconv.Itoa(request.Sequence)
	s.runtimeFiles.mu.Lock()
	session := s.runtimeFiles.sessions[key]
	s.runtimeFiles.mu.Unlock()
	if session == nil {
		if request.Offset != 0 {
			return nil, gerror.New("supplier runtime file sequence is unavailable")
		}
		opened, err := s.consumedFiles.OpenConsumedFieldFile(ctx, runtimeFileScope(runtimeContext), request.FieldKey, request.Sequence)
		if err != nil || opened == nil || opened.Body == nil {
			return nil, gerror.New("supplier runtime file content is unavailable")
		}
		candidate := &supplierRuntimeFileSession{file: opened.File, body: opened.Body}
		var attached bool
		session, attached = s.attachRuntimeFile(contextID, callerPluginID, key, candidate)
		if !attached {
			_ = candidate.body.Close()
			return nil, gerror.New("supplier runtime file content is unavailable")
		}
		if session != candidate {
			_ = candidate.body.Close()
		}
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.offset != request.Offset || session.offset > session.file.SizeBytes {
		return nil, gerror.New("supplier runtime file sequence is unavailable")
	}
	remaining := session.file.SizeBytes - session.offset
	readSize := int64(request.MaximumBytes)
	if remaining < readSize {
		readSize = remaining
	}
	data := make([]byte, int(readSize))
	if readSize > 0 {
		n, err := io.ReadFull(session.body, data)
		if err != nil || int64(n) != readSize {
			_ = session.body.Close()
			session.closed = true
			s.runtimeFiles.remove(key, session)
			return nil, gerror.New("supplier runtime file content is unavailable")
		}
	}
	chunk := &supplierinterfacecap.ConfigurationFileChunk{Sequence: request.Sequence, Offset: request.Offset, Data: data}
	session.offset += readSize
	chunk.EOF = session.offset == session.file.SizeBytes
	if chunk.EOF {
		var probe [1]byte
		if n, err := session.body.Read(probe[:]); n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
			_ = session.body.Close()
			session.closed = true
			s.runtimeFiles.remove(key, session)
			return nil, gerror.New("supplier runtime file content is unavailable")
		}
		_ = session.body.Close()
		session.closed = true
		s.runtimeFiles.remove(key, session)
	}
	return chunk, nil
}

func (s *supplierRuntimeFileStore) remove(key string, target *supplierRuntimeFileSession) {
	s.mu.Lock()
	if s.sessions[key] == target {
		delete(s.sessions, key)
	}
	s.mu.Unlock()
}

func runtimeFileScope(item supplierRuntimeContext) purchaseconfigurationfile.ConsumedFileScope {
	return purchaseconfigurationfile.ConsumedFileScope{
		IntentNo: item.intentNo, OrderNo: item.orderNo, OrderItemNo: item.orderItemNo, ConfigurationSnapshotNo: item.configurationSnapshotNo,
		PluginID: item.callerPluginID, ReleaseVersion: item.releaseVersion, ContractVersion: item.contractVersion, ReleaseDigest: item.releaseDigest,
		DeclarationDigest: item.declarationDigest, Method: item.method, InterfaceInstanceNo: item.interfaceInstanceNo,
		InterfaceConfigurationRevision: item.configurationRevision,
	}
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
