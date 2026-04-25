package cli

import (
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"termtap.dev/internal/model"
	"termtap.dev/internal/tui"
)

func runDemo() {
	events := demoEvents()
	ch := make(chan model.Event, len(events)+8)
	for _, ev := range events {
		ch <- ev
	}

	if err := runTUIFn(ch, tui.Controls{}); err != nil {
		fatalExit(err)
	}
}

func demoEvents() []model.Event {
	base := time.Now().Add(-12 * time.Second)
	request := func(method, host, path string, status int, duration time.Duration, reqBody, respBody string, reqHeaders, respHeaders http.Header) []model.Event {
		id := uuid.New()
		startedAt := base
		base = base.Add(850 * time.Millisecond)

		requestURL := (&url.URL{Scheme: "https", Host: host, Path: path}).String()
		start := model.Event{
			Time: startedAt,
			Type: model.EventTypeRequestStarted,
			Request: model.Request{
				ID:             id,
				Method:         method,
				Host:           host,
				URL:            path,
				RawURL:         requestURL,
				QueryString:    "",
				Pending:        true,
				StartTime:      startedAt,
				RequestData:    []byte(reqBody),
				RequestHeaders: reqHeaders,
			},
		}

		finish := model.Event{
			Time: startedAt.Add(duration),
			Type: model.EventTypeRequestFinished,
			Request: model.Request{
				ID:              id,
				Method:          method,
				Host:            host,
				URL:             path,
				RawURL:          requestURL,
				Status:          status,
				Duration:        duration,
				Pending:         false,
				StartTime:       startedAt,
				RequestData:     []byte(reqBody),
				ResponseData:    []byte(respBody),
				RequestHeaders:  reqHeaders,
				ResponseHeaders: respHeaders,
			},
		}

		return []model.Event{start, finish}
	}

	events := []model.Event{
		{Time: base, Type: model.EventTypeSessionStarted, Body: "demo session started"},
		{Time: base.Add(100 * time.Millisecond), Type: model.EventTypeProxyStarting, Body: "proxy warming up"},
		{Time: base.Add(200 * time.Millisecond), Type: model.EventTypeProxyStarted, Body: "proxy listening on 127.0.0.1:8080"},
		{Time: base.Add(300 * time.Millisecond), Type: model.EventTypeProcessStarting, Body: "starting demo app: go run ."},
		{Time: base.Add(450 * time.Millisecond), Type: model.EventTypeProcessStarted, PID: 48213, Body: "demo app pid 48213"},
		{Time: base.Add(500 * time.Millisecond), Type: model.EventTypeProcessStdout, Body: "tap • 12 requests captured"},
		{Time: base.Add(650 * time.Millisecond), Type: model.EventTypeProcessStdout, Body: "tap • replay mode enabled"},
		{Time: base.Add(800 * time.Millisecond), Type: model.EventTypeProcessStderr, Body: "tap • upstream 429s detected"},
	}

	events = append(events, request(
		"POST",
		"api.stripe.com",
		"/v1/payment_intents",
		200,
		183*time.Millisecond,
		`{"amount":4900,"currency":"usd","payment_method":"pm_card_visa"}`,
		`{"id":"pi_3NtDemo","status":"succeeded"}`,
		http.Header{"Authorization": []string{"Bearer sk_demo_123"}, "Content-Type": []string{"application/json"}},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"GET",
		"api.github.com",
		"/repos/lovable/tap/pulls",
		200,
		245*time.Millisecond,
		"",
		`[{"number":12,"title":"Add demo mode"}]`,
		http.Header{"Accept": []string{"application/vnd.github+json"}},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"POST",
		"api.openai.com",
		"/v1/chat/completions",
		200,
		1832*time.Millisecond,
		`{"model":"gpt-4.1-mini","stream":true}`,
		`{"id":"chatcmpl_demo","choices":[{"delta":{"content":"hello"}}]}`,
		http.Header{"Authorization": []string{"Bearer sk-openai-demo"}},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"GET",
		"api.example.com",
		"/v2/users/42/preferences",
		500,
		2105*time.Millisecond,
		"",
		`{"error":"internal_server_error"}`,
		http.Header{},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"GET",
		"hooks.slack.com",
		"/services/T0B/xxx",
		200,
		312*time.Millisecond,
		"",
		`{"ok":true}`,
		http.Header{},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"PUT",
		"api.example.com",
		"/v2/users/42/preferences",
		404,
		89*time.Millisecond,
		`{"theme":"terminal"}`,
		`{"error":"not_found"}`,
		http.Header{"Content-Type": []string{"application/json"}},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"DELETE",
		"api.stripe.com",
		"/v1/subscriptions/sub_xyz",
		200,
		156*time.Millisecond,
		"",
		`{"status":"canceled"}`,
		http.Header{},
		http.Header{"Content-Type": {"application/json"}},
	)...)
	events = append(events, request(
		"PATCH",
		"api.openai.com",
		"/v1/fine_tuning/jobs",
		429,
		423*time.Millisecond,
		`{"suffix":"demo","hyperparameters":{"n_epochs":3}}`,
		`{"error":"rate_limit_exceeded"}`,
		http.Header{"Authorization": []string{"Bearer sk-openai-demo"}},
		http.Header{"Retry-After": []string{"2"}},
	)...)
	events = append(events, request(
		"POST",
		"api.resend.com",
		"/emails",
		422,
		67*time.Millisecond,
		`{"to":"demo@example.com"}`,
		`{"error":"unprocessable_entity"}`,
		http.Header{"Content-Type": []string{"application/json"}},
		http.Header{"Content-Type": []string{"application/json"}},
	)...)
	events = append(events, request(
		"GET",
		"cdn.jsdelivr.net",
		"/npm/lodash@4.17.21/lodash.min.js",
		200,
		23*time.Millisecond,
		"",
		"/* minified js */",
		http.Header{},
		http.Header{"Content-Type": []string{"application/javascript"}},
	)...)
	events = append(events, model.Event{Time: base.Add(1200 * time.Millisecond), Type: model.EventTypeProcessStdout, Body: "tap • demo ready • press q to quit"})

	return events
}
