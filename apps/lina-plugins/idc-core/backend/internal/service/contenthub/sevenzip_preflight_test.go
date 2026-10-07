// sevenzip_preflight_test.go verifies encrypted and unsafe 7Z archives fail closed.
package contenthub

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"io/fs"
	"sync"
	"testing"
	"time"
)

func TestSevenZipPreflightAcceptsOrdinaryAndRejectsEncryptedStructures(t *testing.T) {
	ordinary := clamdOrdinarySevenZipSample()
	if err := preflightSevenZipReader(bytes.NewReader(ordinary), int64(len(ordinary))); err != nil {
		t.Fatalf("ordinary 7z was rejected: %v", err)
	}
	for _, encoded := range []string{
		clamdEncryptedHeaderSevenZipSample,
		clamdEncryptedCompressedSevenZipSample,
	} {
		archive, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if err = preflightSevenZipReader(bytes.NewReader(archive), int64(len(archive))); err == nil {
			t.Fatal("encrypted 7z structure was accepted")
		}
	}
}

func TestSevenZipEntryPathAndTypeBounds(t *testing.T) {
	for _, test := range []struct {
		name  string
		mode  fs.FileMode
		valid bool
	}{
		{name: "folder/file.txt", valid: true},
		{name: "folder", mode: fs.ModeDir, valid: true},
		{name: "../escape", valid: false},
		{name: "/absolute", valid: false},
		{name: `C:\absolute`, valid: false},
		{name: "folder/\x00file", valid: false},
		{name: "link", mode: fs.ModeSymlink, valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if validSevenZipEntry(test.name, test.mode) != test.valid {
				t.Fatalf("validSevenZipEntry(%q, %v) did not return %v", test.name, test.mode, test.valid)
			}
		})
	}
}

func TestStorageReaderAtHonorsCancellationAndTraversalBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := newStorageReaderAt(ctx, nil, "object", 10)
	if _, err := reader.ReadAt(make([]byte, 1), 0); err == nil {
		t.Fatal("cancelled storage reader accepted a read")
	}
	reader = newStorageReaderAt(context.Background(), nil, "object", 10)
	reader.traversed = reader.traversalBudget
	if err := reader.reserveTraversal(1); err == nil {
		t.Fatal("storage reader exceeded its traversal budget")
	}
}

func TestCancellationReadCloserInterruptsBlockedStorageRead(t *testing.T) {
	body := &blockingReadCloser{closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	reader := newCancellationReadCloser(ctx, body)
	result := make(chan error, 1)
	go func() {
		_, err := reader.Read(make([]byte, 1))
		result <- err
	}()
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled storage read returned success")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled storage read remained blocked")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}

type blockingReadCloser struct {
	closed chan struct{}
	once   sync.Once
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	<-r.closed
	return 0, io.ErrClosedPipe
}

func (r *blockingReadCloser) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}
