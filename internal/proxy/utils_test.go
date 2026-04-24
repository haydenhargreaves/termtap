package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestCanDisplayContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{name: "empty content type", contentType: "", want: false},
		{name: "text type", contentType: "text/plain", want: true},
		{name: "json type", contentType: "application/json", want: true},
		{name: "xml suffix", contentType: "application/problem+xml", want: true},
		{name: "unknown binary", contentType: "application/octet-stream", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := canDisplayContent(tt.contentType); got != tt.want {
				t.Fatalf("canDisplayContent(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

func TestFormatHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers http.Header
		want    string
	}{
		{
			name:    "empty returns none",
			headers: http.Header{},
			want:    "<none>",
		},
		{
			name: "sorts keys stably",
			headers: http.Header{
				"B-Key": {"b1"},
				"A-Key": {"a1", "a2"},
			},
			want: `A-Key="a1,a2", B-Key="b1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatHeaders(tt.headers); got != tt.want {
				t.Fatalf("formatHeaders() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetEndOfUUID(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	if got, want := getEndOfUUID(id), "426614174000"; got != want {
		t.Fatalf("getEndOfUUID() = %q, want %q", got, want)
	}
}

func TestStatusFromUpstreamError(t *testing.T) {
	t.Parallel()

	newReq := func(ctx context.Context) *http.Request {
		reqURL, err := url.Parse("http://example.com")
		if err != nil {
			t.Fatalf("url parse failed: %v", err)
		}
		return (&http.Request{Method: http.MethodGet, URL: reqURL}).WithContext(ctx)
	}

	tests := []struct {
		name string
		req  *http.Request
		resp *http.Response
		err  error
		want int
	}{
		{
			name: "prefers upstream response status",
			req:  newReq(context.Background()),
			resp: &http.Response{StatusCode: http.StatusTeapot},
			err:  errors.New("ignored"),
			want: http.StatusTeapot,
		},
		{
			name: "context canceled maps to bad gateway",
			req: func() *http.Request {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return newReq(ctx)
			}(),
			err:  context.Canceled,
			want: http.StatusBadGateway,
		},
		{
			name: "deadline exceeded maps to gateway timeout",
			req:  newReq(context.Background()),
			err:  context.DeadlineExceeded,
			want: http.StatusGatewayTimeout,
		},
		{
			name: "net timeout maps to gateway timeout",
			req:  newReq(context.Background()),
			err:  timeoutErr{},
			want: http.StatusGatewayTimeout,
		},
		{
			name: "default maps to bad gateway",
			req:  newReq(context.Background()),
			err:  errors.New("dial failed"),
			want: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := statusFromUpstreamError(tt.req, tt.resp, tt.err); got != tt.want {
				t.Fatalf("statusFromUpstreamError() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewUpstreamTransport(t *testing.T) {
	t.Parallel()

	got := newUpstreamTransport()
	transport, ok := got.(*http.Transport)
	if !ok {
		t.Fatalf("newUpstreamTransport() type = %T, want *http.Transport", got)
	}

	if transport.Proxy != nil {
		t.Fatal("newUpstreamTransport() Proxy must be nil")
	}
}

func TestWritePlainHTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		writer     *bufio.Writer
		wantErr    bool
		wantStatus int
	}{
		{
			name:       "writes valid response and flushes",
			status:     http.StatusBadGateway,
			writer:     bufio.NewWriter(&strings.Builder{}),
			wantErr:    false,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:    "returns write error",
			status:  http.StatusBadGateway,
			writer:  bufio.NewWriter(errWriter{}),
			wantErr: true,
		},
		{
			name:    "returns response write error when writer already failed",
			status:  http.StatusBadGateway,
			writer:  bufio.NewWriter(errWriter{}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var sb strings.Builder
			w := tt.writer
			if !tt.wantErr {
				w = bufio.NewWriter(&sb)
			}

			err := writePlainHTTPError(w, tt.status)
			if tt.name == "returns response write error when writer already failed" {
				_ = w.Flush() // set sticky writer error so resp.Write fails immediately
				err = writePlainHTTPError(w, tt.status)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("writePlainHTTPError() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			resp, readErr := http.ReadResponse(bufio.NewReader(strings.NewReader(sb.String())), &http.Request{Method: http.MethodGet})
			if readErr != nil {
				t.Fatalf("ReadResponse() error = %v", readErr)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			if gotCT := resp.Header.Get("Content-Type"); gotCT != "text/plain; charset=utf-8" {
				t.Fatalf("Content-Type = %q, want %q", gotCT, "text/plain; charset=utf-8")
			}

			wantBody := http.StatusText(tt.status)
			if gotCL := resp.Header.Get("Content-Length"); gotCL != strconv.Itoa(len(wantBody)) {
				t.Fatalf("Content-Length = %q, want %q", gotCL, strconv.Itoa(len(wantBody)))
			}

			body, bodyErr := io.ReadAll(resp.Body)
			if bodyErr != nil {
				t.Fatalf("ReadAll(body) error = %v", bodyErr)
			}
			if string(body) != wantBody {
				t.Fatalf("body = %q, want %q", string(body), wantBody)
			}
		})
	}
}
