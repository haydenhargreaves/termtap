package proxy

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type captureConn struct {
	bytes.Buffer
}

func (c *captureConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (c *captureConn) Close() error                       { return nil }
func (c *captureConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *captureConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (c *captureConn) SetDeadline(_ time.Time) error      { return nil }
func (c *captureConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *captureConn) SetWriteDeadline(_ time.Time) error { return nil }

type failWriteConn struct{}

func (failWriteConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (failWriteConn) Write(_ []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (failWriteConn) Close() error                       { return nil }
func (failWriteConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (failWriteConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (failWriteConn) SetDeadline(_ time.Time) error      { return nil }
func (failWriteConn) SetReadDeadline(_ time.Time) error  { return nil }
func (failWriteConn) SetWriteDeadline(_ time.Time) error { return nil }

type trackingBody struct {
	data   *bytes.Reader
	readN  int
	closed bool
}

func (b *trackingBody) Read(p []byte) (int, error) {
	n, err := b.data.Read(p)
	b.readN += n
	return n, err
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestWriteConnectEstablished(t *testing.T) {
	t.Parallel()

	t.Run("writes directly to raw conn", func(t *testing.T) {
		t.Parallel()

		conn := &captureConn{}
		if err := writeConnectEstablished(conn, nil); err != nil {
			t.Fatalf("writeConnectEstablished() error = %v", err)
		}

		if got, want := conn.String(), "HTTP/1.1 200 Connection Established\r\n\r\n"; got != want {
			t.Fatalf("raw write = %q, want %q", got, want)
		}
	})

	t.Run("writes and flushes with buffered readWriter", func(t *testing.T) {
		t.Parallel()

		conn := &captureConn{}
		rw := bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(conn))
		if err := writeConnectEstablished(conn, rw); err != nil {
			t.Fatalf("writeConnectEstablished() error = %v", err)
		}

		if got, want := conn.String(), "HTTP/1.1 200 Connection Established\r\n\r\n"; got != want {
			t.Fatalf("buffered write = %q, want %q", got, want)
		}
	})

	t.Run("returns flush error", func(t *testing.T) {
		t.Parallel()

		rw := bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(errWriter{}))
		err := writeConnectEstablished(&captureConn{}, rw)
		if err == nil {
			t.Fatal("writeConnectEstablished() error = nil, want non-nil")
		}
	})

	t.Run("returns buffered write error when writer already failed", func(t *testing.T) {
		t.Parallel()

		bw := bufio.NewWriter(errWriter{})
		_ = bw.Flush() // set sticky error to force WriteString error path
		rw := bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bw)

		err := writeConnectEstablished(&captureConn{}, rw)
		if err == nil {
			t.Fatal("writeConnectEstablished() error = nil, want non-nil")
		}
	})

	t.Run("returns raw conn write error", func(t *testing.T) {
		t.Parallel()
		err := writeConnectEstablished(failWriteConn{}, nil)
		if err == nil {
			t.Fatal("writeConnectEstablished() error = nil, want non-nil")
		}
	})
}

func TestDiscardAndCloseBody(t *testing.T) {
	t.Parallel()

	t.Run("nil body is safe", func(t *testing.T) {
		t.Parallel()
		discardAndCloseBody(nil)
	})

	t.Run("closes body and discards at most limit", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte("x"), maxDiscardBodyBytes+128)
		body := &trackingBody{data: bytes.NewReader(payload)}

		discardAndCloseBody(body)

		if !body.closed {
			t.Fatal("body was not closed")
		}
		if body.readN != maxDiscardBodyBytes {
			t.Fatalf("bytes read = %d, want %d", body.readN, maxDiscardBodyBytes)
		}
	})
}
