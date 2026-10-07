// clamd_scanner_test.go verifies the bounded clamd protocol and startup self-test.
package contenthub

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClamdScannerStreamsBoundedInputAndMapsStableVerdicts(t *testing.T) {
	tests := []struct {
		name, reply, verdict string
		payload              []byte
		errorExpected        bool
	}{
		{name: "clean", reply: "stream: OK\x00", verdict: ScanVerdictClean},
		{name: "ordinary 7z clean", payload: clamdOrdinarySevenZipSample(), reply: "stream: OK\x00", verdict: ScanVerdictClean},
		{name: "risk", reply: "stream: Eicar-Test-Signature FOUND\x00", verdict: ScanVerdictRisk},
		{name: "encrypted 7z risk signal", payload: clamdOrdinarySevenZipSample(), reply: "stream: Heuristics.Encrypted.7Zip FOUND\x00", verdict: ScanVerdictRisk},
		{name: "daemon error", reply: "stream: database unavailable ERROR\x00", errorExpected: true},
		{name: "incomplete", reply: "stream: OK", errorExpected: true},
		{name: "ambiguous", reply: "stream: UNKNOWN\x00", errorExpected: true},
		{name: "overlong", reply: strings.Repeat("x", clamdReplyLimitBytes) + "\x00", errorExpected: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := test.payload
			if len(payload) == 0 {
				payload = bytes.Repeat([]byte("scan-me"), 20000)
			}
			client, server := net.Pipe()
			t.Cleanup(func() {
				closePipeForTest(t, client)
				closePipeForTest(t, server)
			})
			scanner := testClamdScanner(client, time.Second)
			serverErr := make(chan error, 1)
			go func() {
				serveErr := serveClamdReply(server, int64(len(payload)), test.reply)
				serverErr <- errors.Join(serveErr, normalizePipeCloseError(server.Close()))
			}()
			verdict, err := scanner.Scan(context.Background(), bytes.NewReader(payload), int64(len(payload)))
			if test.errorExpected != (err != nil) || (!test.errorExpected && verdict != test.verdict) {
				t.Fatalf("verdict=%q err=%v", verdict, err)
			}
			if serveErr := <-serverErr; serveErr != nil {
				t.Fatal(serveErr)
			}
		})
	}
}

func TestClamdScannerFailsClosedOnDisconnectTimeoutAndSizeMismatch(t *testing.T) {
	t.Run("disconnect", func(t *testing.T) {
		client, server := net.Pipe()
		scanner := testClamdScanner(client, time.Second)
		closePipeForTest(t, server)
		if _, err := scanner.Scan(context.Background(), bytes.NewReader([]byte("a")), 1); err == nil {
			t.Fatal("disconnect produced a safety verdict")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		client, server := net.Pipe()
		t.Cleanup(func() { closePipeForTest(t, server) })
		scanner := testClamdScanner(client, 30*time.Millisecond)
		copyErr := make(chan error, 1)
		go func() {
			_, err := io.Copy(io.Discard, server)
			copyErr <- normalizePipeCloseError(err)
		}()
		if _, err := scanner.Scan(context.Background(), bytes.NewReader([]byte("a")), 1); err == nil {
			t.Fatal("timeout produced a safety verdict")
		}
		if err := <-copyErr; err != nil {
			t.Fatal(err)
		}
	})
	for _, test := range []struct {
		name     string
		payload  []byte
		expected int64
	}{
		{name: "short", payload: []byte("a"), expected: 2},
		{name: "long", payload: []byte("ab"), expected: 1},
		{name: "over maximum", payload: []byte("a"), expected: maximumDownloadBytes + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, server := net.Pipe()
			t.Cleanup(func() { closePipeForTest(t, server) })
			scanner := testClamdScanner(client, time.Second)
			copyErr := make(chan error, 1)
			go func() {
				_, err := io.Copy(io.Discard, server)
				copyErr <- normalizePipeCloseError(err)
			}()
			_, scanErr := scanner.Scan(context.Background(), bytes.NewReader(test.payload), test.expected)
			closePipeForTest(t, client)
			if scanErr == nil {
				t.Fatal("size mismatch produced a safety verdict")
			}
			if err := <-copyErr; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClamdScannerCancellationClosesBlockedStream(t *testing.T) {
	client, server := net.Pipe()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveClamdReply(server, 1, "")
	}()
	scanner := testClamdScanner(client, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := scanner.Scan(ctx, bytes.NewReader([]byte("a")), 1)
		result <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled scan produced a safety verdict")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled scan did not close the blocked local connection")
	}
	if err := normalizePipeCloseError(<-serverDone); err != nil {
		t.Fatal(err)
	}
}

func TestClamdScannerSelfTestRequiresCleanAndEncryptedVerdicts(t *testing.T) {
	tests := []struct {
		name          string
		replies       []string
		ready         bool
		errorExpected bool
	}{
		{name: "ready", replies: []string{"stream: OK\x00", "stream: Heuristics.Encrypted.7Zip FOUND\x00", "stream: Heuristics.Encrypted.7Zip FOUND\x00", "stream: Heuristics.Encrypted.7Zip FOUND\x00"}, ready: true},
		{name: "ordinary not clean", replies: []string{"stream: UNKNOWN\x00"}, errorExpected: true},
		{name: "encrypted not found", replies: []string{"stream: OK\x00", "stream: OK\x00"}, errorExpected: true},
		{name: "encrypted error", replies: []string{"stream: OK\x00", "stream: scanner ERROR\x00"}, errorExpected: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			scanner := &clamdScanner{config: ClamdConfig{Network: "tcp", Address: "127.0.0.1:3310", Timeout: time.Second, DangerousLimitBytes: 1024}}
			scanner.dial = func(context.Context, string, string) (net.Conn, error) {
				if calls >= len(test.replies) {
					return nil, io.ErrUnexpectedEOF
				}
				client, server := net.Pipe()
				reply := test.replies[calls]
				calls++
				go func() {
					serveErr := serveClamdReplyWithoutExpectedSize(server, reply)
					if closeErr := normalizePipeCloseError(server.Close()); closeErr != nil {
						serveErr = errors.Join(serveErr, closeErr)
					}
					if serveErr != nil {
						t.Errorf("serve self-test response: %v", serveErr)
					}
				}()
				return client, nil
			}
			err := scanner.SelfTest(context.Background())
			if test.errorExpected != (err != nil) || scanner.Ready() != test.ready {
				t.Fatalf("ready=%v err=%v", scanner.Ready(), err)
			}
		})
	}
}

func closePipeForTest(t *testing.T, connection net.Conn) {
	t.Helper()
	if err := normalizePipeCloseError(connection.Close()); err != nil {
		t.Errorf("close test pipe: %v", err)
	}
}

func normalizePipeCloseError(err error) error {
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}

func TestNewClamdScannerRejectsNonLocalEndpoint(t *testing.T) {
	if _, err := NewClamdScanner(ClamdConfig{Network: "tcp", Address: "192.0.2.10:3310", Timeout: time.Minute, DangerousLimitBytes: 1024}); err == nil {
		t.Fatal("outbound clamd endpoint was accepted")
	}
	if _, err := NewClamdScanner(ClamdConfig{Network: "tcp", Address: "clamd.internal:3310", Timeout: time.Minute, DangerousLimitBytes: 1024}); err == nil {
		t.Fatal("hostname clamd endpoint was accepted")
	}
}

func testClamdScanner(connection net.Conn, timeout time.Duration) *clamdScanner {
	return &clamdScanner{
		config: ClamdConfig{Network: "tcp", Address: "127.0.0.1:3310", Timeout: timeout, DangerousLimitBytes: maximumDangerousDownloadBytes},
		dial: func(context.Context, string, string) (net.Conn, error) {
			return connection, nil
		},
	}
}

func serveClamdReply(connection net.Conn, expected int64, reply string) error {
	command := make([]byte, len("zINSTREAM\x00"))
	if _, err := io.ReadFull(connection, command); err != nil {
		return err
	}
	var received int64
	length := make([]byte, 4)
	for {
		if _, err := io.ReadFull(connection, length); err != nil {
			return err
		}
		size := binary.BigEndian.Uint32(length)
		if size == 0 {
			break
		}
		if _, err := io.CopyN(io.Discard, connection, int64(size)); err != nil {
			return err
		}
		received += int64(size)
	}
	if received != expected {
		return io.ErrUnexpectedEOF
	}
	_, err := io.WriteString(connection, reply)
	return err
}

func serveClamdReplyWithoutExpectedSize(connection net.Conn, reply string) error {
	command := make([]byte, len("zINSTREAM\x00"))
	if _, err := io.ReadFull(connection, command); err != nil {
		return err
	}
	length := make([]byte, 4)
	for {
		if _, err := io.ReadFull(connection, length); err != nil {
			return err
		}
		size := binary.BigEndian.Uint32(length)
		if size == 0 {
			break
		}
		if _, err := io.CopyN(io.Discard, connection, int64(size)); err != nil {
			return err
		}
	}
	_, err := io.WriteString(connection, reply)
	return err
}
