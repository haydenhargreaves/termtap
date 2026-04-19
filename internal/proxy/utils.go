package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
)

var validContentTypes = []string{
	"application/graphql",
	"application/javascript",
	"application/json",
	"application/x-www-form-urlencoded",
	"application/xml",
	"+json",
	"+xml",
}

func canDisplayContent(contentType string) bool {
	if contentType == "" {
		return false
	}

	contentType = strings.ToLower(contentType)
	if strings.HasPrefix(contentType, "text/") {
		return true
	}

	for _, t := range validContentTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}

	return false
}

// NOTE: Currently unused, will be reference for the future header rendering
func formatHeaders(headers http.Header) string {
	if len(headers) == 0 {
		return "<none>"
	}

	parts := make([]string, 0, len(headers))
	for key, values := range headers {
		parts = append(parts, fmt.Sprintf("%s=%q", key, strings.Join(values, ",")))
	}
	sort.Strings(parts)

	return strings.Join(parts, ", ")
}

func getEndOfUUID(id uuid.UUID) string {
	return id.String()[24:]
}

// BUG: Not sure if this actually works, seems to favor the 502
func statusFromUpstreamError(req *http.Request, resp *http.Response, err error) int {
	if resp != nil {
		return resp.StatusCode
	}

	if errors.Is(req.Context().Err(), context.Canceled) {
		return http.StatusBadGateway
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return http.StatusGatewayTimeout
	}

	return http.StatusBadGateway
}

func newUpstreamTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return transport
}

func writePlainHTTPError(w *bufio.Writer, status int) error {
	resp := &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(http.StatusText(status))),
		ContentLength: int64(len(http.StatusText(status))),
		Close:         false,
	}
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(http.StatusText(status))))
	if err := resp.Write(w); err != nil {
		return err
	}
	return w.Flush()
}
