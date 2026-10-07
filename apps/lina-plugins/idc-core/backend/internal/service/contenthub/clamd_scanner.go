// clamd_scanner.go streams content to a local clamd daemon and classifies its reply.
package contenthub

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	clamdStreamChunkBytes                  = 64 * 1024
	clamdReplyLimitBytes                   = 4096
	ScanVerdictClean                       = "clean"
	ScanVerdictRisk                        = "risk"
	maximumDangerousDownloadBytes          = int64(2 * 1024 * 1024 * 1024)
	clamdEncryptedHeaderSevenZipSample     = "N3q8ryccAAQfQXHHwAAAAAAAAAAoAAAAAAAAALn7J1qgRMFFatX0e5vxiYNkCc0bOOTUmvi+KatqrtUARthD6ik2mQ2RgdM8A3HxtXiuzmUYq53Om8X6sE3kZ+A1br2Ylv2nvh3rUMcWgf1i+MC8UXkcAiQZme6XopM71m+OeNlu8lfFYkKp/EA4SKNOVdtinaYnj0Y6pRJQJhRTVRWX9XgSnN3fd0sFwKmndH7i0WMdM0gRC4Y6c4wSthZkV1kllHvYvgcInoSnXefRgKBVaQ34kCZCq0xo2g90+JSboO8XBiABCYCgAAcLAQABJAbxBwEKUwf0wep1D5nnYwyAlgoB8PBMOwAA"
	clamdEncryptedCompressedSevenZipSample = "N3q8ryccAAQZPwq/fQAAAAAAAAAgAAAAAAAAADfp0uy/yFjkFdadbTq6EDSfSOaTAACBMweuD87ysgwHsMPa919FipdTgilRmAEQAhLTPSSWedwNTLs1pIFAusGbXPphDu/LJSNENG45EQ990gX2H8aWbJqUvttxWjo0kngSGUx1uxodfPul72dl4+qZuzA/mOdWwQ4P3fgEgQAAABcGEAEJbQAHCwEAASMDAQEFXQAQAAAMdgoBVofKcwAA"
	clamdEncryptedStoredSevenZipSample     = "N3q8ryccAAQJRb72mAAAAAAAAAAhAAAAAAAAAEU/IIvjnJsKEpbatjlvGk9BMbj9iuKadV5m9Pg26TxnRg7o4wAAgTMHrjGZYgmQFdYeIaV7ES8WpAwAbbtTM8GtDTYiR7/Thg1SNAMb/okpcts0IKdJr17OfWdUGNWng/fUX549X2TvHdNqyfJa5RchbFamCOz9r2TrjZ6zsMY6S2TLRm1F2R9XBxq2dbf9Sh9wYVV7aYHbVQAAABcGIAEJeAAHCwEAASMDAQEFXQAQAAAMgJYKASZWKokAAA=="
)

type Scanner interface {
	Configured() bool
	Ready() bool
	DangerousLimitBytes() int64
	Timeout() time.Duration
	SelfTest(context.Context) error
	Scan(context.Context, io.Reader, int64) (string, error)
}

type ClamdConfig struct {
	Network             string
	Address             string
	Timeout             time.Duration
	DangerousLimitBytes int64
}

type clamdScanner struct {
	config ClamdConfig
	dial   func(context.Context, string, string) (net.Conn, error)
	ready  atomic.Bool
}

func NewClamdScanner(config ClamdConfig) (Scanner, error) {
	if config.Network == "" && config.Address == "" {
		if config.Timeout <= 0 || config.DangerousLimitBytes != 0 {
			return nil, gerror.New("content scanner timeout must be positive")
		}
		return &clamdScanner{config: config}, nil
	}
	if config.Timeout <= 0 || config.DangerousLimitBytes < 0 || config.DangerousLimitBytes > maximumDangerousDownloadBytes || !validLocalClamdEndpoint(config.Network, config.Address) {
		return nil, gerror.New("content scanner requires a valid local clamd endpoint")
	}
	dialer := &net.Dialer{}
	return &clamdScanner{
		config: config,
		dial:   dialer.DialContext,
	}, nil
}

func validLocalClamdEndpoint(network, address string) bool {
	switch network {
	case "unix":
		return strings.HasPrefix(address, "/") && len(address) > 1 && !strings.Contains(address, "\\")
	case "tcp":
		host, port, err := net.SplitHostPort(address)
		if err != nil || port == "" {
			return false
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	default:
		return false
	}
}

func (s *clamdScanner) Configured() bool {
	return s != nil && s.config.Network != "" && s.config.Address != "" && s.dial != nil
}

// Ready reports whether the deployment proved both complete ordinary archive
// scanning and encrypted archive detection for its configured size boundary.
func (s *clamdScanner) Ready() bool {
	return s != nil && s.Configured() && s.ready.Load()
}

// DangerousLimitBytes returns the deployment-tested upload boundary.
func (s *clamdScanner) DangerousLimitBytes() int64 {
	if s == nil {
		return 0
	}
	return s.config.DangerousLimitBytes
}

func (s *clamdScanner) Timeout() time.Duration {
	if s == nil {
		return 0
	}
	return s.config.Timeout
}

// SelfTest proves that the local daemon returns a complete clean verdict for a
// controlled ordinary archive and a risk verdict for a controlled encrypted
// archive. Raw daemon details are intentionally not exposed to callers.
func (s *clamdScanner) SelfTest(ctx context.Context) error {
	if s == nil {
		return gerror.New("content scanner is unavailable")
	}
	s.ready.Store(false)
	if !s.Configured() || s.DangerousLimitBytes() < 1 {
		return gerror.New("content scanner is unavailable")
	}
	ordinary := clamdOrdinarySevenZipSample()
	verdict, err := s.Scan(ctx, bytes.NewReader(ordinary), int64(len(ordinary)))
	if err != nil || verdict != ScanVerdictClean {
		return gerror.Wrap(errors.Join(err, gerror.New("ordinary archive did not receive a clean verdict")), "verify local clamd ordinary archive behavior")
	}
	// These compact fixtures are derived from github.com/bodgit/sevenzip's
	// BSD-3-Clause testdata and cover encrypted headers plus encrypted compressed
	// and stored file streams. All forms must fail closed before enablement.
	for _, encoded := range []string{
		clamdEncryptedHeaderSevenZipSample,
		clamdEncryptedCompressedSevenZipSample,
		clamdEncryptedStoredSevenZipSample,
	} {
		encrypted, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			return gerror.Wrap(decodeErr, "decode controlled encrypted archive")
		}
		verdict, err = s.Scan(ctx, bytes.NewReader(encrypted), int64(len(encrypted)))
		if err != nil || verdict != ScanVerdictRisk {
			return gerror.Wrap(errors.Join(err, gerror.New("encrypted archive did not receive a risk verdict")), "verify local clamd encrypted archive behavior")
		}
	}
	s.ready.Store(true)
	return nil
}

func (s *clamdScanner) Scan(ctx context.Context, body io.Reader, expectedSize int64) (verdict string, resultErr error) {
	if ctx == nil || ctx.Err() != nil || body == nil || expectedSize < 1 || expectedSize > maximumDownloadBytes || !s.Configured() {
		return "", gerror.New("content scanner is unavailable")
	}
	scanCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	connection, err := s.dial(scanCtx, s.config.Network, s.config.Address)
	if err != nil {
		return "", gerror.Wrap(err, "connect local clamd")
	}
	cancelClose := make(chan error, 1)
	stopCancelClose := context.AfterFunc(scanCtx, func() {
		cancelClose <- connection.Close()
	})
	defer func() {
		if stopCancelClose() {
			resultErr = errors.Join(resultErr, connection.Close())
			return
		}
		closeErr := <-cancelClose
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			resultErr = errors.Join(resultErr, closeErr)
		}
		resultErr = errors.Join(resultErr, scanCtx.Err())
	}()
	if deadline, ok := scanCtx.Deadline(); ok {
		if err = connection.SetDeadline(deadline); err != nil {
			return "", gerror.Wrap(err, "bound local clamd request")
		}
	}
	if _, err = io.WriteString(connection, "zINSTREAM\x00"); err != nil {
		return "", gerror.Wrap(err, "start local clamd stream")
	}
	limited := &io.LimitedReader{R: body, N: expectedSize}
	buffer := make([]byte, clamdStreamChunkBytes)
	length := make([]byte, 4)
	for limited.N > 0 {
		read, readErr := limited.Read(buffer)
		if read > 0 {
			binary.BigEndian.PutUint32(length, uint32(read))
			if err = writeFull(connection, length); err == nil {
				err = writeFull(connection, buffer[:read])
			}
			if err != nil {
				return "", gerror.Wrap(err, "stream candidate to local clamd")
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) && limited.N == 0 {
				break
			}
			return "", gerror.Wrap(readErr, "read candidate for local clamd")
		}
		if read == 0 {
			return "", gerror.New("candidate stream ended without a complete scan input")
		}
	}
	if limited.N != 0 {
		return "", gerror.New("candidate stream ended before the declared size")
	}
	var extra [1]byte
	if read, readErr := body.Read(extra[:]); read != 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
		return "", gerror.New("candidate stream exceeds the declared size")
	}
	binary.BigEndian.PutUint32(length, 0)
	if err = writeFull(connection, length); err != nil {
		return "", gerror.Wrap(err, "finish local clamd stream")
	}
	reply, err := readClamdReply(connection)
	if err != nil {
		return "", err
	}
	if reply == "stream: OK" {
		return ScanVerdictClean, nil
	}
	if strings.HasPrefix(reply, "stream: ") && strings.HasSuffix(reply, " FOUND") {
		signature := strings.TrimSuffix(strings.TrimPrefix(reply, "stream: "), " FOUND")
		if signature != "" && len(signature) <= 256 && !strings.ContainsAny(signature, "\r\n\x00") {
			return ScanVerdictRisk, nil
		}
	}
	return "", gerror.New("local clamd did not return a complete safety verdict")
}

func clamdOrdinarySevenZipSample() []byte {
	nextHeader := []byte{0x01, 0x00}
	startHeader := make([]byte, 20)
	binary.LittleEndian.PutUint64(startHeader[8:16], uint64(len(nextHeader)))
	binary.LittleEndian.PutUint32(startHeader[16:20], crc32.ChecksumIEEE(nextHeader))
	archive := make([]byte, 0, 32+len(nextHeader))
	archive = append(archive, '7', 'z', 0xbc, 0xaf, 0x27, 0x1c, 0, 4)
	checksum := make([]byte, 4)
	binary.LittleEndian.PutUint32(checksum, crc32.ChecksumIEEE(startHeader))
	archive = append(archive, checksum...)
	archive = append(archive, startHeader...)
	return append(archive, nextHeader...)
}

func writeFull(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		written, err := writer.Write(value)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		value = value[written:]
	}
	return nil
}

func readClamdReply(reader io.Reader) (string, error) {
	buffered := bufio.NewReaderSize(reader, clamdReplyLimitBytes)
	record, err := buffered.ReadString(0)
	if err != nil || len(record) < 2 || len(record) > clamdReplyLimitBytes || record[len(record)-1] != 0 {
		return "", gerror.New("local clamd response is incomplete")
	}
	record = strings.TrimSuffix(record, "\x00")
	if record == "" || strings.ContainsAny(record, "\r\n") {
		return "", gerror.New("local clamd response is invalid")
	}
	return record, nil
}
