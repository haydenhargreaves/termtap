package proxy

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
)

func TestNewBodyPreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		wantEnabled bool
	}{
		{name: "text content enabled", contentType: "text/plain", wantEnabled: true},
		{name: "json content enabled", contentType: "application/json", wantEnabled: true},
		{name: "binary content disabled", contentType: "application/octet-stream", wantEnabled: false},
		{name: "empty content type disabled", contentType: "", wantEnabled: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := newBodyPreview(tt.contentType)
			if p.enabled != tt.wantEnabled {
				t.Fatalf("newBodyPreview(%q).enabled = %v, want %v", tt.contentType, p.enabled, tt.wantEnabled)
			}
		})
	}
}

func TestBodyPreviewWriteAndPreview(t *testing.T) {
	t.Parallel()

	t.Run("nil receiver is safe", func(t *testing.T) {
		t.Parallel()
		var p *bodyPreview
		p.Write([]byte("abc"))
	})

	t.Run("disabled preview ignores data", func(t *testing.T) {
		t.Parallel()
		p := &bodyPreview{enabled: false}
		p.Write([]byte("abc"))
		if got := string(p.Preview()); got != "" {
			t.Fatalf("Preview() = %q, want empty", got)
		}
	})

	t.Run("empty write does nothing", func(t *testing.T) {
		t.Parallel()
		p := &bodyPreview{enabled: true}
		p.Write(nil)
		if got := string(p.Preview()); got != "" {
			t.Fatalf("Preview() = %q, want empty", got)
		}
	})

	t.Run("escapes newlines", func(t *testing.T) {
		t.Parallel()
		p := &bodyPreview{enabled: true}
		p.Write([]byte("a\nb"))
		if got, want := string(p.Preview()), `a\nb`; got != want {
			t.Fatalf("Preview() = %q, want %q", got, want)
		}
	})

	t.Run("truncates at max preview bytes and appends ellipsis", func(t *testing.T) {
		t.Parallel()
		p := &bodyPreview{enabled: true}
		p.Write([]byte(strings.Repeat("a", maxPreviewBytes)))
		p.Write([]byte("b"))

		got := string(p.Preview())
		if !strings.HasSuffix(got, "...") {
			t.Fatalf("Preview() must end with ellipsis when truncated: %q", got[len(got)-10:])
		}
		if len(got) != maxPreviewBytes+3 {
			t.Fatalf("len(Preview()) = %d, want %d", len(got), maxPreviewBytes+3)
		}
	})
}

func TestWrapBufferedConn(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	t.Run("returns original conn when readWriter nil", func(t *testing.T) {
		t.Parallel()
		got := wrapBufferedConn(client, nil)
		if got != client {
			t.Fatal("wrapBufferedConn should return original conn when readWriter is nil")
		}
	})

	t.Run("read uses buffered readWriter", func(t *testing.T) {
		t.Parallel()
		rw := bufio.NewReadWriter(bufio.NewReader(strings.NewReader("xyz")), bufio.NewWriter(io.Discard))
		got := wrapBufferedConn(client, rw)

		buf := make([]byte, 3)
		n, err := got.Read(buf)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if n != 3 || string(buf) != "xyz" {
			t.Fatalf("Read() = (%d, %q), want (3, %q)", n, string(buf), "xyz")
		}
	})
}

func TestPreviewReadCloserRead(t *testing.T) {
	t.Parallel()

	preview := newBodyPreview("text/plain")
	rc := &previewReadCloser{
		ReadCloser: io.NopCloser(strings.NewReader("hello\nworld")),
		preview:    preview,
	}

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if got, want := string(data), "hello\nworld"; got != want {
		t.Fatalf("read content = %q, want %q", got, want)
	}
	if got, want := string(preview.Preview()), `hello\nworld`; got != want {
		t.Fatalf("preview = %q, want %q", got, want)
	}
}
