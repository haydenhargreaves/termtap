package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"termtap.dev/internal/model"
)

// NOTE: Much of this code is AI generated, and is not expected to make it into production

const maxPreviewBytes = 1024

func proxyHandler(ch chan<- model.Message) http.Handler {
	transport := http.DefaultTransport

	// TODO: This should be wired into the main channel, but that will require a model package
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			http.Error(w, "CONNECT is not supported yet", http.StatusNotImplemented)
			ch <- model.Message{
				Type: model.MessageTypeWarn,
				Body: fmt.Sprintf("CONNECT is not supported: %s", req.Host),
			}
			return
		}

		if req.URL.Scheme == "" || req.URL.Host == "" {
			http.Error(w, "request must use absolute-form URLs through the proxy", http.StatusBadRequest)
			ch <- model.Message{
				Type: model.MessageTypeWarn,
				Body: fmt.Sprintf("rejected non-proxy request %s %s", req.Method, req.URL.String()),
			}
			return
		}

		start := time.Now()
		// requestPreview, err := readAndRestoreBody(&req.Body)
		// if err != nil {
		// 	http.Error(w, "failed to read request body", http.StatusBadRequest)
		// 	log.Printf("!! read request body %s %s: %v", req.Method, req.URL.String(), err)
		// 	return
		// }

		outReq := req.Clone(req.Context())
		outReq.RequestURI = ""
		ch <- model.Message{
			Type: model.MessageTypeRequestStarted,
			Body: fmt.Sprintf("-> %s %s", outReq.Method, outReq.URL.String()),
		}

		resp, err := transport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			ch <- model.Message{
				Type: model.MessageTypeRequestFailed,
				Body: fmt.Sprintf("upstream error for %s %s: %v", outReq.Method, outReq.URL.String(), err),
			}
			return
		}
		defer resp.Body.Close()

		// responsePreview, err := readAndRestoreBody(&resp.Body)
		// if err != nil {
		// 	http.Error(w, "bad gateway", http.StatusBadGateway)
		// 	log.Printf("!! read response body %s %s: %v", outReq.Method, outReq.URL.String(), err)
		// 	return
		// }

		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			ch <- model.Message{
				Type: model.MessageTypeRequestFailed,
				Body: fmt.Sprintf("write response body %s %s: %v", outReq.Method, outReq.URL.String(), err),
			}
			return
		}

		ch <- model.Message{
			Type: model.MessageTypeRequestFinished,
			Body: fmt.Sprintf("<- %s %s %d %s",
				outReq.Method,
				outReq.URL.String(),
				resp.StatusCode,
				time.Since(start).Round(time.Millisecond),
			),
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
