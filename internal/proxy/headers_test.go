package proxy

import (
	"net/http"
	"reflect"
	"testing"
)

func TestStripHopByHopHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   http.Header
		want http.Header
	}{
		{
			name: "removes static hop-by-hop headers",
			in: http.Header{
				"Connection":        {"keep-alive"},
				"Keep-Alive":        {"timeout=5"},
				"Proxy-Connection":  {"keep-alive"},
				"Transfer-Encoding": {"chunked"},
				"Upgrade":           {"websocket"},
				"X-Custom":          {"ok"},
			},
			want: http.Header{
				"X-Custom": {"ok"},
			},
		},
		{
			name: "removes headers listed in connection with spaces and commas",
			in: http.Header{
				"Connection":     {" keep-alive, X-Foo ,X-Bar"},
				"Keep-Alive":     {"timeout=5"},
				"X-Foo":          {"foo"},
				"X-Bar":          {"bar"},
				"Content-Type":   {"application/json"},
				"Content-Length": {"12"},
			},
			want: http.Header{
				"Content-Type":   {"application/json"},
				"Content-Length": {"12"},
			},
		},
		{
			name: "keeps unrelated headers when no connection header",
			in: http.Header{
				"Accept":        {"*/*"},
				"Authorization": {"Bearer abc"},
			},
			want: http.Header{
				"Accept":        {"*/*"},
				"Authorization": {"Bearer abc"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stripHopByHopHeaders(tt.in)

			if !reflect.DeepEqual(tt.in, tt.want) {
				t.Fatalf("stripHopByHopHeaders() = %#v, want %#v", tt.in, tt.want)
			}
		})
	}
}

func TestStripHopByHopHeaders_NilHeader(t *testing.T) {
	t.Parallel()

	stripHopByHopHeaders(nil)
}

func TestRedactHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input http.Header
		want  http.Header
	}{
		{
			name: "redacts sensitive canonical headers",
			input: http.Header{
				"Authorization":       {"Bearer token123"},
				"Cookie":              {"session=abc"},
				"Proxy-Authorization": {"Basic abc"},
				"Set-Cookie":          {"a=1"},
				"X-Api-Key":           {"secret"},
			},
			want: http.Header{
				"Authorization":       {"[REDACTED]"},
				"Cookie":              {"[REDACTED]"},
				"Proxy-Authorization": {"[REDACTED]"},
				"Set-Cookie":          {"[REDACTED]"},
				"X-Api-Key":           {"[REDACTED]"},
			},
		},
		{
			name: "leaves non-sensitive headers untouched",
			input: http.Header{
				"Content-Type": {"application/json"},
				"X-Trace-ID":   {"trace-1"},
			},
			want: http.Header{
				"Content-Type": {"application/json"},
				"X-Trace-ID":   {"trace-1"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := redactHeaders(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("redactHeaders() = %#v, want %#v", got, tt.want)
			}

			got.Set("Content-Type", "modified")
			if reflect.DeepEqual(tt.input, got) {
				t.Fatal("redactHeaders() appears to mutate input or return aliased map")
			}
		})
	}
}

func TestCopyHeaders(t *testing.T) {
	t.Parallel()

	src := http.Header{
		"X-Multi":      {"a", "b", "c"},
		"Content-Type": {"application/json"},
	}
	dest := http.Header{
		"Existing": {"keep"},
	}

	copyHeaders(src, dest)

	want := http.Header{
		"Existing":     {"keep"},
		"X-Multi":      {"a", "b", "c"},
		"Content-Type": {"application/json"},
	}

	if !reflect.DeepEqual(dest, want) {
		t.Fatalf("copyHeaders() dest = %#v, want %#v", dest, want)
	}
}
