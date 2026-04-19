package proxy

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"termtap.dev/internal/model"
)

const connectIdleTimeout = 30 * time.Second

func proxyHandler(ch chan<- model.Event, ca *CertificateAuthority, ps *model.ProxyServer) http.Handler {
	transport := newUpstreamTransport()

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodConnect {
			handleConnect(w, req, ch, transport, ca, ps)
			return
		}

		if req.URL.Scheme == "" || req.URL.Host == "" {
			http.Error(w, "request must use absolute-form URLs through the proxy", http.StatusBadRequest)
			ch <- model.Event{
				Time: time.Now().Local(),
				Type: model.EventTypeWarn,
				Body: fmt.Sprintf("rejected non-proxy request %s %s", req.Method, req.URL.String()),
			}
			return
		}

		resp, request, responsePreview, err := roundTripCapturedRequest(req, transport, ch, "", false)
		if err != nil {
			status := statusFromUpstreamError(req, resp, err)

			http.Error(w, http.StatusText(status), status)
			failRequest(ch, request, status, fmt.Sprintf("upstream error: %v", err))
			return
		}
		defer resp.Body.Close()

		copyHeaders(resp.Header, w.Header())
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			request.ResponseData = responsePreview.Preview()
			failRequest(ch, request, resp.StatusCode, fmt.Sprintf("failed to write response body: %v", err))
			return
		}

		request.ResponseData = responsePreview.Preview()
		finishRequest(ch, request, resp.StatusCode)
	})
}

func handleConnect(w http.ResponseWriter, req *http.Request, ch chan<- model.Event, transport http.RoundTripper, ca *CertificateAuthority, ps *model.ProxyServer) {
	start := time.Now()

	request := newConnectRequest(req, start)
	startRequest(ch, request)

	target := req.Host
	if !strings.Contains(target, ":") {
		target = net.JoinHostPort(target, "443")
	}

	if ca == nil {
		http.Error(w, "HTTPS interception unavailable", http.StatusBadGateway)
		failRequest(ch, request, http.StatusBadGateway, "HTTPS interception certificate authority is unavailable")
		return
	}

	leafCert, err := ca.CertificateForHost(target)
	if err != nil {
		http.Error(w, "failed to prepare interception certificate", http.StatusBadGateway)
		failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("failed to mint interception certificate for %s: %v", target, err))
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "proxy does not support hijacking", http.StatusInternalServerError)
		failRequest(ch, request, http.StatusInternalServerError, "CONNECT hijack is unavailable")
		return
	}

	clientConn, readWriter, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "failed to hijack connection", http.StatusInternalServerError)
		failRequest(ch, request, http.StatusInternalServerError, fmt.Sprintf("CONNECT hijack failed: %v", err))
		return
	}
	trackConnection(ps, clientConn)
	defer func() {
		untrackConnection(ps, clientConn)
		_ = clientConn.Close()
	}()

	if err := writeConnectEstablished(clientConn, readWriter); err != nil {
		failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("CONNECT setup failed: %v", err))
		return
	}

	mitmConn := wrapBufferedConn(clientConn, readWriter)
	tlsConn := tls.Server(mitmConn, &tls.Config{
		Certificates: []tls.Certificate{*leafCert},
		MinVersion:   tls.VersionTLS12,
	})
	defer tlsConn.Close()

	_ = clientConn.SetDeadline(time.Now().Add(connectIdleTimeout))
	if err := tlsConn.Handshake(); err != nil {
		failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("TLS handshake with client failed: %v", err))
		return
	}
	_ = clientConn.SetDeadline(time.Time{})

	reader := bufio.NewReader(tlsConn)
	writer := bufio.NewWriter(tlsConn)

	for {
		_ = clientConn.SetReadDeadline(time.Now().Add(connectIdleTimeout))
		innerReq, err := http.ReadRequest(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				finishRequest(ch, request, http.StatusOK)
				return
			}
			failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("failed to read decrypted HTTPS request: %v", err))
			return
		}
		_ = clientConn.SetReadDeadline(time.Time{})

		resp, captured, responsePreview, err := roundTripCapturedRequest(innerReq, transport, ch, target, true)
		if err != nil {
			discardAndCloseBody(innerReq.Body)
			status := statusFromUpstreamError(innerReq, resp, err)
			_ = clientConn.SetWriteDeadline(time.Now().Add(connectIdleTimeout))
			if writeErr := writePlainHTTPError(writer, status); writeErr != nil {
				failRequest(ch, captured, status, fmt.Sprintf("upstream error: %v", err))
				failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("failed to write HTTPS error response: %v", writeErr))
				return
			}
			_ = clientConn.SetWriteDeadline(time.Time{})
			failRequest(ch, captured, status, fmt.Sprintf("upstream error: %v", err))
			failRequest(ch, request, status, fmt.Sprintf("closing CONNECT tunnel after upstream error: %v", err))
			return
		}

		_ = clientConn.SetWriteDeadline(time.Now().Add(connectIdleTimeout))
		if err := resp.Write(writer); err != nil {
			resp.Body.Close()
			captured.ResponseData = responsePreview.Preview()
			failRequest(ch, captured, resp.StatusCode, fmt.Sprintf("failed to write HTTPS response: %v", err))
			failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("failed to write HTTPS response: %v", err))
			return
		}

		if err := writer.Flush(); err != nil {
			_ = clientConn.SetWriteDeadline(time.Time{})
			resp.Body.Close()
			captured.ResponseData = responsePreview.Preview()
			failRequest(ch, captured, resp.StatusCode, fmt.Sprintf("failed to flush HTTPS response: %v", err))
			failRequest(ch, request, http.StatusBadGateway, fmt.Sprintf("failed to flush HTTPS response: %v", err))
			return
		}
		_ = clientConn.SetWriteDeadline(time.Time{})

		captured.ResponseData = responsePreview.Preview()
		finishRequest(ch, captured, resp.StatusCode)
		shouldClose := innerReq.Close || resp.Close
		resp.Body.Close()
		if shouldClose {
			finishRequest(ch, request, http.StatusOK)
			return
		}
	}
}
