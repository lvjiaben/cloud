// Upload regressions exercise real PostgreSQL state and local filesystem object assembly.

package contenthub

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/storagecap"
)

// TestUploadChunkExtendsExpiryPostgreSQL guards the timestamp parameter used by real chunk uploads.
func TestUploadChunkExtendsExpiryPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	service.storage = &contentHubChunkStorageStub{root: t.TempDir()}
	item, err := service.Create(ctx, WriteInput{Type: "download", Visibility: "public", Title: "Chunk expiry", Body: "Upload regression"})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("%PDF-1.7\n")
	upload, err := service.CreateUpload(ctx, UploadCreateInput{ContentNo: item.ContentNo, FileName: "guide.pdf", SizeBytes: int64(len(content)), ContentVersion: item.Version})
	if err != nil {
		t.Fatal(err)
	}
	before, err := contentDatabaseNow(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := service.UploadChunk(ctx, upload.UploadNo, 0, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("confirm real PostgreSQL chunk: %v", err)
	}
	stored, err := service.GetUpload(ctx, upload.UploadNo)
	if err != nil {
		t.Fatal(err)
	}
	if chunk.ConfirmedBytes != int64(len(content)) || !chunk.Complete || chunk.Version != 2 || stored.ConfirmedBytes != int64(len(content)) || stored.Version != 2 {
		t.Fatalf("chunk progress was not persisted: chunk=%+v stored=%+v", chunk, stored)
	}
	if delta := stored.ExpiresAt.Sub(before); delta < 24*time.Hour-time.Second || delta > 24*time.Hour+time.Minute {
		t.Fatalf("expiry must extend by 24 hours, got %s", delta)
	}
	if _, err = service.UploadChunk(ctx, upload.UploadNo, 0, bytes.NewReader(content)); err == nil {
		t.Fatal("duplicate chunk must not advance progress")
	}
	candidate, err := service.CompleteUpload(ctx, CompleteInput{UploadNo: upload.UploadNo, ExpectedSizeBytes: int64(len(content)), Version: stored.Version})
	if err != nil {
		t.Fatalf("assemble on local filesystem without directory collision: %v", err)
	}
	generation, err := database.GetValue(ctx, `SELECT candidate_generation FROM plugin_idc_core_content_file WHERE file_no=?`, candidate.FileNo)
	if err != nil || generation.Int64() != 1 {
		t.Fatalf("candidate generation=%v err=%v", generation, err)
	}
	if candidate.ContentKind != "pdf" || candidate.SizeBytes != int64(len(content)) {
		t.Fatalf("unexpected assembled candidate: %+v", candidate)
	}
}

// contentHubChunkStorageStub isolates object I/O while retaining real PostgreSQL upload state.
type contentHubChunkStorageStub struct {
	contentHubStorageStub
	root string
}

func (s *contentHubChunkStorageStub) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	path := filepath.Join(s.root, filepath.FromSlash(input.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(file, input.Body)
	return &storagecap.PutOutput{}, errors.Join(copyErr, file.Close())
}

func (s *contentHubChunkStorageStub) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	file, err := os.Open(filepath.Join(s.root, filepath.FromSlash(input.Path)))
	if err != nil {
		return nil, err
	}
	return &storagecap.GetOutput{Found: true, Body: file}, nil
}

func (s *contentHubChunkStorageStub) Delete(_ context.Context, input storagecap.DeleteInput) error {
	return os.Remove(filepath.Join(s.root, filepath.FromSlash(input.Path)))
}

// TestAssemblyRejectClosesProducer verifies early storage failure cannot leave a blocked pipe writer.
func TestAssemblyRejectClosesProducer(t *testing.T) {
	closed := make(chan struct{})
	storage := &contentHubRejectWriteStorageStub{closed: closed}
	service := &serviceImpl{storage: storage}
	if err := service.assembleChunkObjects(context.Background(), &uploadRow{ObjectRef: "final", DeclaredSize: 1}, []uploadPart{{ObjectRef: "part"}}); err == nil {
		t.Fatal("storage rejection was ignored")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("assembly producer retained its object after an early write failure")
	}
}

// contentHubRejectWriteStorageStub rejects before consuming the pipe and observes producer cleanup.
type contentHubRejectWriteStorageStub struct {
	contentHubStorageStub
	closed chan struct{}
}

func (s *contentHubRejectWriteStorageStub) Put(context.Context, storagecap.PutInput) (*storagecap.PutOutput, error) {
	return nil, errors.New("storage rejected write")
}
func (s *contentHubRejectWriteStorageStub) Get(context.Context, storagecap.GetInput) (*storagecap.GetOutput, error) {
	return &storagecap.GetOutput{Found: true, Body: &contentHubObservedBody{Reader: strings.NewReader("x"), closed: s.closed}}, nil
}

// contentHubObservedBody records the point where an assembly producer releases its source.
type contentHubObservedBody struct {
	io.Reader
	closed chan struct{}
}

func (r *contentHubObservedBody) Close() error { close(r.closed); return nil }
