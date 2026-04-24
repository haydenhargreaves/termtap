package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

var sensitiveHeaders = map[string]struct{}{
	"Authorization":       {},
	"Cookie":              {},
	"Proxy-Authorization": {},
	"Set-Cookie":          {},
	"X-Api-Key":           {},
}

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// Remove headers that are only required for client<->proxy and proxy<->server communication.
// Otherwise known as hop-by-hop headers.
func stripHopByHopHeaders(headers http.Header) {
	if headers == nil {
		return
	}

	connectionValues := append([]string(nil), headers.Values("Connection")...)
	for _, key := range hopByHopHeaders {
		headers.Del(key)
	}

	for _, value := range connectionValues {
		for key := range strings.SplitSeq(value, ",") {
			headers.Del(strings.TrimSpace(key))
		}
	}
}

func captureRequestHeaders(req *http.Request) http.Header {
	if req == nil {
		return http.Header{}
	}

	headers := req.Header.Clone()

	host := strings.TrimSpace(req.Host)
	if host == "" && req.URL != nil {
		host = strings.TrimSpace(req.URL.Host)
	}
	if host != "" {
		headers.Set("Host", host)
	}

	if req.ContentLength > 0 && headers.Get("Content-Length") == "" {
		headers.Set("Content-Length", fmt.Sprintf("%d", req.ContentLength))
	}

	if len(req.TransferEncoding) > 0 && headers.Get("Transfer-Encoding") == "" {
		headers.Set("Transfer-Encoding", strings.Join(req.TransferEncoding, ", "))
	}

	return headers
}

func captureResponseHeaders(resp *http.Response) http.Header {
	if resp == nil {
		return http.Header{}
	}

	headers := resp.Header.Clone()

	if resp.ContentLength > 0 && headers.Get("Content-Length") == "" {
		headers.Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}

	if len(resp.TransferEncoding) > 0 && headers.Get("Transfer-Encoding") == "" {
		headers.Set("Transfer-Encoding", strings.Join(resp.TransferEncoding, ", "))
	}

	return headers
}

// Return a new set of headers that has sensitive headers redacted.
//
// TODO: Maybe use '***' length of header?
func redactHeaders(headers http.Header) http.Header {
	clone := headers.Clone()
	for key := range clone {
		if _, ok := sensitiveHeaders[http.CanonicalHeaderKey(key)]; ok {
			clone.Set(key, "[REDACTED]")
		}
	}
	return clone
}

func copyHeaders(src, dest http.Header) {
	for key, values := range src {
		for _, value := range values {
			dest.Add(key, value)
		}
	}
}
