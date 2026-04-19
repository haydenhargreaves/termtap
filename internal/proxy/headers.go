package proxy

import (
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
// Otherwise known as hop-by-hop headers. We do not want to show these to users since they are
// used only for internal functioning for the proxy server.
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
