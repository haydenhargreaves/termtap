package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

// NOTE: Much of this code is AI generated, and is not expected to make it into production

const maxPreviewBytes = 1024

func proxyHandler(ch chan<- model.Event) http.Handler {
	transport := http.DefaultTransport

	// TODO: This should be wired into the main channel, but that will require a model package
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			http.Error(w, "CONNECT is not supported yet", http.StatusNotImplemented)
			ch <- model.Event{
				Type: model.EventTypeWarn,
				Body: fmt.Sprintf("CONNECT is not supported: %s", req.Host),
			}
			return
		}

		if req.URL.Scheme == "" || req.URL.Host == "" {
			http.Error(w, "request must use absolute-form URLs through the proxy", http.StatusBadRequest)
			ch <- model.Event{
				Type: model.EventTypeWarn,
				Body: fmt.Sprintf("rejected non-proxy request %s %s", req.Method, req.URL.String()),
			}
			return
		}

		start := time.Now()

		request := model.Request{
			ID:           uuid.New(),
			ResponseData: []byte{},
			RequestData:  []byte{},
			URL:          "",
			Status:       -1,
			Method:       "",
			Duration:     0,
			Pending:      true,
			Failed:       false,
			StartTime:    start,
		}

		requestPreview, err := readAndRestoreBody(&req.Body)
		if err != nil {
			ch <- model.Event{
				Type:    model.EventTypeWarn,
				Body:    fmt.Sprintf("(%s) failed to read request body", request.ID),
				Request: request,
			}
		} else {
			request.RequestData = []byte(requestPreview)
		}

		outReq := req.Clone(req.Context())
		outReq.RequestURI = ""

		request.URL = outReq.URL.Path
		request.QueryString = outReq.URL.RawQuery
		request.QueryMap = outReq.URL.Query()
		request.Host = outReq.Host
		request.Method = outReq.Method
		request.RequestHeaders = outReq.Header
		request.RawURL = outReq.URL.String()

		ch <- model.Event{
			Type:    model.EventTypeRequestStarted,
			Body:    fmt.Sprintf("-> %+v", request),
			Request: request,
		}

		resp, err := transport.RoundTrip(outReq)
		if err != nil {
			status := statusFromUpstreamError(req, resp, err)

			http.Error(w, http.StatusText(status), status)
			request.Pending = false
			request.Failed = true
			request.Duration = time.Since(start).Round(time.Microsecond)
			request.Status = status

			ch <- model.Event{
				Type:    model.EventTypeRequestFailed,
				Body:    fmt.Sprintf("upstream error for %s %s: %v", outReq.Method, outReq.URL.String(), err),
				Request: request,
			}
			return
		}
		defer resp.Body.Close()

		responsePreview, err := readAndRestoreBody(&resp.Body)
		if err != nil {
			ch <- model.Event{
				Type:    model.EventTypeWarn,
				Body:    fmt.Sprintf("(%s) failed to read response body", request.ID),
				Request: request,
			}
		} else {
			request.ResponseData = []byte(responsePreview)
		}

		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			request.Pending = false
			request.Failed = true
			request.Duration = time.Since(start).Round(time.Microsecond)
			request.Status = resp.StatusCode

			ch <- model.Event{
				Type: model.EventTypeRequestFailed,
				Body: fmt.Sprintf("write response body %s %s: %v", outReq.Method, outReq.URL.String(), err),
			}
			return
		}

		request.Duration = time.Since(start).Round(time.Microsecond)
		request.Status = resp.StatusCode
		request.ResponseHeaders = resp.Header
		request.Pending = false

		ch <- model.Event{
			Type:    model.EventTypeRequestFinished,
			Body:    fmt.Sprintf("<- %+v %s", request, formatHeaders(resp.Request.Header)),
			Request: request,
		}
	})
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func readAndRestoreBody(body *io.ReadCloser) (string, error) {
	if body == nil || *body == nil {
		return "", nil
	}

	payload, err := io.ReadAll(*body)
	if err != nil {
		return "", err
	}

	*body = io.NopCloser(bytes.NewReader(payload))

	preview := payload
	if len(preview) > maxPreviewBytes {
		preview = preview[:maxPreviewBytes]
	}

	text := strings.ReplaceAll(string(preview), "\n", "\\n")
	if len(payload) > maxPreviewBytes {
		text += "..."
	}

	return text, nil
}

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
