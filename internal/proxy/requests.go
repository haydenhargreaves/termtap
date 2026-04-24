package proxy

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

func roundTripCapturedRequest(req *http.Request, transport http.RoundTripper, ch chan<- model.Event, defaultHost string, interceptedTLS bool) (*http.Response, model.Request, *bodyPreview, error) {
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

	outReq := req.Clone(req.Context())
	outReq.RequestURI = ""
	if interceptedTLS {
		if outReq.URL.Scheme == "" {
			outReq.URL.Scheme = "https"
		}
		if outReq.URL.Host == "" {
			outReq.URL.Host = defaultHost
		}
		if outReq.Host == "" {
			outReq.Host = defaultHost
		}
	}
	capturedRequestHeaders := captureRequestHeaders(outReq)
	stripHopByHopHeaders(outReq.Header)
	requestPreview := newBodyPreview(outReq.Header.Get("Content-Type"))
	if outReq.Body != nil {
		outReq.Body = &previewReadCloser{ReadCloser: outReq.Body, preview: requestPreview}
	}

	request.URL = outReq.URL.Path
	request.QueryString = outReq.URL.RawQuery
	request.QueryMap = outReq.URL.Query()
	request.Host = outReq.Host
	request.Method = outReq.Method
	request.RequestHeaders = redactHeaders(capturedRequestHeaders)
	request.RawURL = outReq.URL.String()
	if request.RawURL == "" {
		request.RawURL = outReq.Host + outReq.URL.RequestURI()
	}

	startRequest(ch, request)

	resp, err := transport.RoundTrip(outReq)
	request.RequestData = requestPreview.Preview()
	if err != nil {
		return resp, request, nil, err
	}

	capturedResponseHeaders := captureResponseHeaders(resp)
	stripHopByHopHeaders(resp.Header)
	responsePreview := newBodyPreview(resp.Header.Get("Content-Type"))
	if resp.Body != nil {
		resp.Body = &previewReadCloser{ReadCloser: resp.Body, preview: responsePreview}
	}

	request.ResponseHeaders = redactHeaders(capturedResponseHeaders)
	return resp, request, responsePreview, nil
}

func newConnectRequest(req *http.Request, start time.Time) model.Request {
	// CONNECT requests do not have as much data, which is why we use Host for most of the pieces
	return model.Request{
		ID:           uuid.New(),
		ResponseData: []byte{},
		RequestData:  []byte{},
		URL:          req.Host,
		RawURL:       req.Host,
		Host:         req.Host,
		Status:       -1,
		Method:       req.Method,
		Duration:     0,
		Pending:      true,
		Failed:       false,
		StartTime:    start,
	}
}

func finishRequest(ch chan<- model.Event, request model.Request, status int) {
	request.Pending = false
	request.Failed = false
	request.Status = status
	request.Duration = time.Since(request.StartTime).Round(time.Microsecond)

	ch <- model.Event{
		Time:    time.Now().Local(),
		Type:    model.EventTypeRequestFinished,
		Body:    fmt.Sprintf("(%s) %s %s %d %dms", getEndOfUUID(request.ID), request.Method, request.RawURL, request.Status, request.Duration.Milliseconds()),
		Request: request,
	}
}

func failRequest(ch chan<- model.Event, request model.Request, status int, body string) {
	request.Pending = false
	request.Failed = true
	request.Status = status
	request.Duration = time.Since(request.StartTime).Round(time.Microsecond)

	ch <- model.Event{
		Time:    time.Now().Local(),
		Type:    model.EventTypeRequestFailed,
		Body:    fmt.Sprintf("(%s) %s", getEndOfUUID(request.ID), body),
		Request: request,
	}
}

func startRequest(ch chan<- model.Event, request model.Request) {
	ch <- model.Event{
		Time:    time.Now().Local(),
		Type:    model.EventTypeRequestStarted,
		Body:    fmt.Sprintf("(%s) %s %s", getEndOfUUID(request.ID), request.Method, request.RawURL),
		Request: request,
	}
}
