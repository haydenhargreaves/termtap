package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

const maxPreviewBytes = 1024

func test() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if err := runCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	case "proxy":
		if err := runProxy(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  tap run -- <command> [args...]")
	fmt.Fprintln(os.Stderr, "  tap proxy [-listen 127.0.0.1:8080]")
}

func runCommand(args []string) error {
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	listenAddr := runFlags.String("listen", "127.0.0.1:0", "proxy listen address")
	runFlags.SetOutput(io.Discard)

	if err := runFlags.Parse(args); err != nil {
		return err
	}

	commandArgs := runFlags.Args()
	if len(commandArgs) == 0 {
		return errors.New("run requires a command after `--`")
	}
	if commandArgs[0] == "--" {
		commandArgs = commandArgs[1:]
	}
	if len(commandArgs) == 0 {
		return errors.New("run requires a command after `--`")
	}

	server, proxyURL, err := startProxy(*listenAddr)
	if err != nil {
		return err
	}
	defer shutdownServer(server)

	log.Printf("proxy listening on %s", proxyURL)

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = withProxyEnv(os.Environ(), proxyURL)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	forwardSignals(cmd.Process)

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("wait for command: %w", err)
	}

	return nil
}

func runProxy(args []string) error {
	proxyFlags := flag.NewFlagSet("proxy", flag.ExitOnError)
	listenAddr := proxyFlags.String("listen", "127.0.0.1:8080", "proxy listen address")
	if err := proxyFlags.Parse(args); err != nil {
		return err
	}

	server, proxyURL, err := startProxy(*listenAddr)
	if err != nil {
		return err
	}
	defer shutdownServer(server)

	log.Printf("proxy listening on %s", proxyURL)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	<-stop

	return nil
}

func startProxy(listenAddr string) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, "", fmt.Errorf("listen on %s: %w", listenAddr, err)
	}

	server := &http.Server{Handler: newForwardProxy()}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("proxy server error: %v", err)
		}
	}()

	proxyURL := "http://" + listener.Addr().String()
	return server, proxyURL, nil
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func withProxyEnv(env []string, proxyURL string) []string {
	filtered := make([]string, 0, len(env)+5)
	for _, entry := range env {
		if hasEnvKey(entry, "HTTP_PROXY") || hasEnvKey(entry, "http_proxy") || hasEnvKey(entry, "HTTPS_PROXY") || hasEnvKey(entry, "https_proxy") || hasEnvKey(entry, "ALL_PROXY") || hasEnvKey(entry, "all_proxy") || hasEnvKey(entry, "NO_PROXY") || hasEnvKey(entry, "no_proxy") {
			continue
		}
		filtered = append(filtered, entry)
	}

	filtered = append(filtered,
		"HTTP_PROXY="+proxyURL,
		"http_proxy="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"https_proxy="+proxyURL,
		"ALL_PROXY="+proxyURL,
		"all_proxy="+proxyURL,
		"NO_PROXY=",
		"no_proxy=",
	)

	return filtered
}

func hasEnvKey(entry, key string) bool {
	return strings.HasPrefix(entry, key+"=")
}

func forwardSignals(process *os.Process) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		for sig := range ch {
			_ = process.Signal(sig)
		}
	}()
}

func newForwardProxy() http.Handler {
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
		requestPreview, err := readAndRestoreBody(&req.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			log.Printf("!! read request body %s %s: %v", req.Method, req.URL.String(), err)
			return
		}

		outReq := req.Clone(req.Context())
		outReq.RequestURI = ""

		log.Printf(
			"-> %s %s\n   request headers: %s\n   request body: %q",
			outReq.Method,
			outReq.URL.String(),
			formatHeaders(outReq.Header),
			requestPreview,
		)

		resp, err := transport.RoundTrip(outReq)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			log.Printf("!! upstream error for %s %s: %v", outReq.Method, outReq.URL.String(), err)
			return
		}
		defer resp.Body.Close()

		responsePreview, err := readAndRestoreBody(&resp.Body)
		if err != nil {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			log.Printf("!! read response body %s %s: %v", outReq.Method, outReq.URL.String(), err)
			return
		}

		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("!! write response body %s %s: %v", outReq.Method, outReq.URL.String(), err)
			return
		}

		log.Printf(
			"<- %s %s %d %s\n   response headers: %s\n   response body: %q",
			outReq.Method,
			outReq.URL.String(),
			resp.StatusCode,
			time.Since(startedAt).Round(time.Millisecond),
			formatHeaders(resp.Header),
			responsePreview,
		)
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
