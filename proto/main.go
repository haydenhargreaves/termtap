package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

func main() {
	if err := parseArgs(); err != nil {
		panic(err)
	}
}

func parseArgs() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("Must use this right")
	}

	if os.Args[1] != "run" || os.Args[2] != "--" {
		return fmt.Errorf("Must use this right")
	}

	cmd := os.Args[3:]
	return run(cmd)
}

func run(cmd []string) error {
	fmt.Printf("%+v\n", cmd)

	server, url, err := proxy()
	if err != nil {
		return err
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	println(url)

	env := []string{
		"HTTP_PROXY=" + url,
		"http_proxy=" + url,
		"HTTPS_PROXY=" + url,
		"https_proxy=" + url,
		"ALL_PROXY=" + url,
		"all_proxy=" + url,
		"NO_PROXY=",
		"no_proxy=",
	}

	proc := exec.Command(cmd[0], cmd[1:]...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	proc.Env = append(os.Environ(), env...)

	if err := proc.Start(); err != nil {
		return err
	}

	if err := proc.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("wait for command: %w", err)
	}

	return nil
}

func proxy() (*http.Server, string, error) {
	addr := "127.0.0.1:8080"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", err
	}

	server := &http.Server{Handler: handler()}

	go func() {
		if err := server.Serve(listener); err != nil {
			fmt.Printf("%q", err)
		}
	}()

	url := "http://" + listener.Addr().String()
	return server, url, nil
}

func handler() http.Handler {
	transport := http.DefaultTransport

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			http.Error(w, "CONNECT is not supported yet", http.StatusNotImplemented)
			log.Printf("!! CONNECT %s not supported", req.Host)
			return
		}

		if req.URL.Scheme == "" || req.URL.Host == "" {
			http.Error(w, "request must use absolute-form URLs through the proxy", http.StatusBadRequest)
			log.Printf("!! rejected non-proxy request %s %s", req.Method, req.URL.String())
			return
		}

		startedAt := time.Now()
		id := uuid.New().String()
		// requestPreview, err := readAndRestoreBody(&req.Body)
		// if err != nil {
		// 	http.Error(w, "failed to read request body", http.StatusBadRequest)
		// 	log.Printf("!! read request body %s %s: %v", req.Method, req.URL.String(), err)
		// 	return
		// }

		outReq := req.Clone(req.Context())
		outReq.RequestURI = ""

		log.Printf(
			"[%s] -> %s %s\n",
			id,
			outReq.Method,
			outReq.URL.String(),
			// formatHeaders(outReq.Header),
			// requestPreview,
		)

		resp, err := transport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			log.Printf("!! upstream error for %s %s: %v", outReq.Method, outReq.URL.String(), err)
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
			log.Printf("!! write response body %s %s: %v", outReq.Method, outReq.URL.String(), err)
			return
		}

		log.Printf(
			"[%s] <- %s %s %d %s\n",
			id,
			outReq.Method,
			outReq.URL.String(),
			resp.StatusCode,
			time.Since(startedAt).Round(time.Millisecond),
		)
	})
}

const maxPreviewBytes = 1024

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
