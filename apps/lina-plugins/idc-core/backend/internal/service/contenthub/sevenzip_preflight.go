// sevenzip_preflight.go validates 7Z structure through bounded object-storage reads.
package contenthub

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/bodgit/sevenzip"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
)

const (
	sevenZipMaximumEntries         = 4096
	sevenZipMaximumPathBytes       = 1024
	sevenZipMaximumExpansionRatio  = int64(100)
	sevenZipMaximumExpandedBytes   = int64(64 * 1024 * 1024 * 1024)
	sevenZipMinimumTraversalBudget = int64(64 * 1024 * 1024)
	sevenZipMaximumTraversalFactor = int64(4)
	sevenZipMaximumReaderAtCalls   = 8192
)

type sevenZipStorage interface {
	Get(context.Context, storagecap.GetInput) (*storagecap.GetOutput, error)
}

// preflightSevenZip validates archive structure through bounded storage reads.
// It never copies the archive to local disk or expands complete file contents.
func preflightSevenZip(ctx context.Context, storage sevenZipStorage, objectRef string, size int64) (resultErr error) {
	readerAt := newStorageReaderAt(ctx, storage, objectRef, size)
	defer func() {
		resultErr = errors.Join(resultErr, readerAt.Close())
	}()
	if err := preflightSevenZipReader(readerAt, size); err != nil {
		return bizerr.NewCode(CodeUploadRejected)
	}
	return nil
}

func preflightSevenZipReader(readerAt io.ReaderAt, archiveSize int64) error {
	if readerAt == nil || archiveSize < 1 || archiveSize > maximumDangerousDownloadBytes {
		return gerror.New("7z archive size is outside the bounded preflight")
	}
	archive, err := sevenzip.NewReader(readerAt, archiveSize)
	if err != nil {
		return gerror.Wrap(err, "parse 7z archive")
	}
	if len(archive.File) > sevenZipMaximumEntries {
		return gerror.New("7z archive has too many entries")
	}
	expandedLimit := archiveSize * sevenZipMaximumExpansionRatio
	if archiveSize > math.MaxInt64/sevenZipMaximumExpansionRatio || expandedLimit > sevenZipMaximumExpandedBytes {
		expandedLimit = sevenZipMaximumExpandedBytes
	}
	if expandedLimit < archiveSize {
		return gerror.New("7z archive expansion limit is invalid")
	}
	var expanded uint64
	streamChecked := make(map[int]struct{}, len(archive.File))
	for _, file := range archive.File {
		if file == nil || !validSevenZipEntry(file.Name, file.Mode()) {
			return gerror.New("7z archive entry is unsafe")
		}
		if file.UncompressedSize > math.MaxInt64 || expanded > math.MaxUint64-file.UncompressedSize {
			return gerror.New("7z archive expanded size overflows")
		}
		expanded += file.UncompressedSize
		if expanded > uint64(expandedLimit) {
			return gerror.New("7z archive expansion exceeds the safety boundary")
		}
		if file.Mode().IsDir() || file.UncompressedSize == 0 {
			continue
		}
		if _, checked := streamChecked[file.Stream]; checked {
			continue
		}
		streamChecked[file.Stream] = struct{}{}
		if err = probeSevenZipEntry(file); err != nil {
			return err
		}
	}
	return nil
}

func validSevenZipEntry(name string, mode fs.FileMode) bool {
	if name == "" || len(name) > sevenZipMaximumPathBytes || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return false
	}
	for _, value := range name {
		if value < 0x20 || value == 0x7f {
			return false
		}
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	cleaned := path.Clean(normalized)
	// Archive paths are portable: Unix filepath.VolumeName does not detect
	// Windows drives, including drive-relative paths such as C:payload.
	windowsDrive := len(normalized) >= 2 && normalized[1] == ':' &&
		((normalized[0] >= 'A' && normalized[0] <= 'Z') || (normalized[0] >= 'a' && normalized[0] <= 'z'))
	if windowsDrive || strings.HasPrefix(normalized, "/") || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || filepath.VolumeName(name) != "" {
		return false
	}
	entryType := mode.Type()
	return entryType == 0 || entryType == fs.ModeDir
}

func probeSevenZipEntry(file *sevenzip.File) (resultErr error) {
	reader, err := file.Open()
	if err != nil {
		return gerror.Wrap(err, "open 7z archive entry")
	}
	defer func() {
		resultErr = errors.Join(resultErr, reader.Close())
	}()
	var first [1]byte
	read, err := reader.Read(first[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return gerror.Wrap(err, "probe 7z archive entry")
	}
	if read != 1 {
		return gerror.New("7z archive entry is incomplete")
	}
	return nil
}

type storageReaderAt struct {
	size            int64
	traversalBudget int64
	traversed       int64
	calls           int
	position        int64
	body            io.ReadCloser
	open            func() (*storagecap.GetOutput, error)
	done            <-chan struct{}
	contextErr      func() error
	mu              sync.Mutex
}

func newStorageReaderAt(ctx context.Context, storage sevenZipStorage, objectRef string, size int64) *storageReaderAt {
	budget := sevenZipMinimumTraversalBudget
	if size > 0 && size <= math.MaxInt64/sevenZipMaximumTraversalFactor && size*sevenZipMaximumTraversalFactor > budget {
		budget = size * sevenZipMaximumTraversalFactor
	}
	return &storageReaderAt{
		size: size, traversalBudget: budget,
		open: func() (*storagecap.GetOutput, error) {
			object, err := storage.Get(ctx, storagecap.GetInput{Path: objectRef})
			if err == nil && object != nil && object.Body != nil {
				object.Body = newCancellationReadCloser(ctx, object.Body)
			}
			return object, err
		},
		done: ctx.Done(), contextErr: ctx.Err,
	}
}

func (r *storageReaderAt) ReadAt(buffer []byte, offset int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.cancellationError(); err != nil {
		return 0, err
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	if offset < 0 || offset >= r.size || int64(len(buffer)) > r.size-offset {
		return 0, io.EOF
	}
	r.calls++
	if r.calls > sevenZipMaximumReaderAtCalls {
		return 0, gerror.New("7z archive exceeded the read-call boundary")
	}
	if r.body == nil || offset < r.position {
		if err := r.closeLocked(); err != nil {
			return 0, err
		}
		object, err := r.open()
		if err != nil || object == nil || !object.Found || object.Body == nil {
			return 0, gerror.Wrap(errors.Join(err, gerror.New("7z archive object is unavailable")), "open 7z archive object")
		}
		r.body = object.Body
		r.position = 0
	}
	if offset > r.position {
		if err := r.consumeLocked(offset - r.position); err != nil {
			return 0, err
		}
	}
	if err := r.reserveTraversal(int64(len(buffer))); err != nil {
		return 0, err
	}
	read, err := io.ReadFull(&contextReader{done: r.done, contextErr: r.contextErr, reader: r.body}, buffer)
	r.position += int64(read)
	if err != nil {
		return read, err
	}
	return read, nil
}

func (r *storageReaderAt) consumeLocked(size int64) error {
	if err := r.reserveTraversal(size); err != nil {
		return err
	}
	written, err := io.CopyN(io.Discard, &contextReader{done: r.done, contextErr: r.contextErr, reader: r.body}, size)
	r.position += written
	if err != nil {
		return gerror.Wrap(err, "seek within 7z archive object")
	}
	return nil
}

func (r *storageReaderAt) reserveTraversal(size int64) error {
	if size < 0 || r.traversed > r.traversalBudget-size {
		return gerror.New("7z archive exceeded the storage traversal boundary")
	}
	r.traversed += size
	return nil
}

func (r *storageReaderAt) cancellationError() error {
	select {
	case <-r.done:
		return r.contextErr()
	default:
		return nil
	}
}

func (r *storageReaderAt) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeLocked()
}

func (r *storageReaderAt) closeLocked() error {
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

type contextReader struct {
	done       <-chan struct{}
	contextErr func() error
	reader     io.Reader
}

type cancellationReadCloser struct {
	body     io.ReadCloser
	stop     func() bool
	done     chan struct{}
	closeErr error
	once     sync.Once
}

func newCancellationReadCloser(ctx context.Context, body io.ReadCloser) *cancellationReadCloser {
	reader := &cancellationReadCloser{body: body, done: make(chan struct{})}
	reader.stop = context.AfterFunc(ctx, reader.finishClose)
	return reader
}

func (r *cancellationReadCloser) Read(buffer []byte) (int, error) {
	return r.body.Read(buffer)
}

func (r *cancellationReadCloser) Close() error {
	if r.stop != nil {
		r.stop()
	}
	r.finishClose()
	<-r.done
	return r.closeErr
}

func (r *cancellationReadCloser) finishClose() {
	r.once.Do(func() {
		r.closeErr = r.body.Close()
		close(r.done)
	})
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.done:
		return 0, r.contextErr()
	default:
	}
	return r.reader.Read(buffer)
}
