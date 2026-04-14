package main

// This is a runable example which will spawn two servers, one we can access
// which hits the other and response with the data provided.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {
	upstreamHost, err := findNonLoopbackIPv4()
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := startUpstream(upstreamHost); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := startFrontend(upstreamHost); err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}

func startFrontend(upstreamHost string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(frontendHTML))
	})

	mux.HandleFunc("/echo", func(w http.ResponseWriter, req *http.Request) {
		client := &http.Client{Timeout: parseTimeout(req.URL.Query().Get("timeoutMs"))}

		switch req.Method {
		case http.MethodGet:
			message := req.URL.Query().Get("message")
			upstreamURL := fmt.Sprintf(
				"http://%s:3001/echo?message=%s&code=%s&fail=%s&sleepMs=%s",
				upstreamHost,
				url.QueryEscape(message),
				url.QueryEscape(req.URL.Query().Get("code")),
				url.QueryEscape(req.URL.Query().Get("fail")),
				url.QueryEscape(req.URL.Query().Get("sleepMs")),
			)

			resp, err := client.Get(upstreamURL)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(body)
		case http.MethodPost:
			body, err := io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if !json.Valid(body) {
				http.Error(w, "invalid JSON payload", http.StatusBadRequest)
				return
			}

			upstreamURL := fmt.Sprintf(
				"http://%s:3001/echo?code=%s&fail=%s&sleepMs=%s",
				upstreamHost,
				url.QueryEscape(req.URL.Query().Get("code")),
				url.QueryEscape(req.URL.Query().Get("fail")),
				url.QueryEscape(req.URL.Query().Get("sleepMs")),
			)

			upstreamReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(body))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			upstreamReq.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(upstreamReq)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			upstreamBody, err := io.ReadAll(resp.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(upstreamBody)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("frontend UI on http://127.0.0.1:3000")
	log.Printf("frontend GET example:  http://127.0.0.1:3000/echo?message=hello&code=201&sleepMs=200")
	log.Printf("frontend POST example: curl -i -X POST 'http://127.0.0.1:3000/echo?code=202&sleepMs=200' -H 'content-type: application/json' -d '{\"message\":\"hello\"}'")
	log.Printf("frontend timeout example: http://127.0.0.1:3000/echo?message=late&sleepMs=4000&timeoutMs=1000")
	log.Printf("frontend failure examples: fail=true, fail=drop, fail=timeout, fail=status")
	log.Printf("frontend calls upstream at http://%s:3001/echo", upstreamHost)
	return http.ListenAndServe("127.0.0.1:3000", mux)
}

func startUpstream(upstreamHost string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, req *http.Request) {
		code := parseStatusCode(req.URL.Query().Get("code"))
		time.Sleep(parseSleep(req.URL.Query().Get("sleepMs")))
		if handleFailureMode(w, req, req.URL.Query().Get("fail"), code) {
			return
		}

		switch req.Method {
		case http.MethodGet:
			message := req.URL.Query().Get("message")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(code)
			_, _ = w.Write([]byte(message))
		case http.MethodPost:
			body, err := io.ReadAll(req.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(code)
			_, _ = w.Write(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Printf("upstream listening on http://%s:3001/echo?message=hello&code=201", upstreamHost)
	log.Printf("upstream POST example: curl -i -X POST 'http://%s:3001/echo?code=202&sleepMs=200' -H 'content-type: application/json' -d '{\"message\":\"hello\"}'", upstreamHost)
	return http.ListenAndServe(":3001", mux)
}

const frontendHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Echo JSON Demo</title>
  <style>
    body {
      margin: 0;
      font-family: ui-sans-serif, sans-serif;
      background: #f5f6f8;
      color: #111827;
      display: grid;
      place-items: center;
      min-height: 100vh;
    }
    main {
      width: min(700px, 92vw);
      background: #ffffff;
      border: 1px solid #d1d5db;
      border-radius: 12px;
      padding: 20px;
      box-shadow: 0 10px 30px rgba(17, 24, 39, 0.08);
    }
    h1 {
      margin-top: 0;
      font-size: 1.25rem;
    }
    label {
      display: block;
      margin: 10px 0 6px;
      font-weight: 600;
    }
    textarea, input {
      width: 100%;
      box-sizing: border-box;
      padding: 10px;
      border: 1px solid #d1d5db;
      border-radius: 8px;
      font-size: 0.95rem;
    }
    textarea {
      min-height: 140px;
      font-family: ui-monospace, monospace;
    }
    .row {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 10px;
    }
    button {
      margin-top: 12px;
      width: 100%;
      padding: 10px;
      border: 0;
      border-radius: 8px;
      background: #0f766e;
      color: #ffffff;
      font-weight: 700;
      cursor: pointer;
    }
    pre {
      margin: 12px 0 0;
      background: #111827;
      color: #f9fafb;
      padding: 12px;
      border-radius: 8px;
      overflow: auto;
      min-height: 80px;
    }
  </style>
</head>
<body>
  <main>
    <h1>Echo JSON Through Frontend</h1>
    <form id="echo-form">
      <label for="payload">JSON payload</label>
      <textarea id="payload">{"message":"hello from form"}</textarea>
      <div class="row">
        <div>
          <label for="code">Status code (optional)</label>
          <input id="code" placeholder="200">
        </div>
        <div>
          <label for="fail">Fail mode (optional)</label>
          <input id="fail" placeholder="false | drop | timeout | status">
        </div>
      </div>
      <div class="row">
        <div>
          <label for="sleepMs">Upstream sleep ms</label>
          <input id="sleepMs" placeholder="0">
        </div>
        <div>
          <label for="timeoutMs">Frontend timeout ms</label>
          <input id="timeoutMs" placeholder="5000">
        </div>
      </div>
      <button type="submit">Send JSON</button>
    </form>
    <pre id="result">Waiting for request...</pre>
  </main>
  <script>
    const form = document.getElementById("echo-form");
    const payloadInput = document.getElementById("payload");
    const codeInput = document.getElementById("code");
    const failInput = document.getElementById("fail");
    const sleepInput = document.getElementById("sleepMs");
    const timeoutInput = document.getElementById("timeoutMs");
    const result = document.getElementById("result");

    form.addEventListener("submit", async (event) => {
      event.preventDefault();

      try {
        JSON.parse(payloadInput.value);
      } catch (err) {
        result.textContent = "invalid JSON: " + err.message;
        return;
      }

      const params = new URLSearchParams();
      if (codeInput.value.trim()) {
        params.set("code", codeInput.value.trim());
      }
      if (failInput.value.trim()) {
        params.set("fail", failInput.value.trim());
      }
      if (sleepInput.value.trim()) {
        params.set("sleepMs", sleepInput.value.trim());
      }
      if (timeoutInput.value.trim()) {
        params.set("timeoutMs", timeoutInput.value.trim());
      }

      const query = params.toString();
      const url = query ? "/echo?" + query : "/echo";

      try {
        const resp = await fetch(url, {
          method: "POST",
          headers: {"Content-Type": "application/json"},
          body: payloadInput.value,
        });
        const body = await resp.text();
        result.textContent = "status: " + resp.status + "\n" + body;
      } catch (err) {
        result.textContent = "request failed: " + err.message;
      }
    });
  </script>
</body>
</html>
`

func handleFailureMode(w http.ResponseWriter, req *http.Request, raw string, requestedCode int) bool {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" || mode == "false" || mode == "0" || mode == "no" {
		return false
	}

	switch mode {
	case "true", "drop":
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "drop mode not supported by server", http.StatusInternalServerError)
			return true
		}

		conn, _, err := hj.Hijack()
		if err != nil {
			http.Error(w, "failed to drop connection", http.StatusInternalServerError)
			return true
		}
		_ = conn.Close()
		return true
	case "timeout", "hang":
		<-req.Context().Done()
		return true
	case "status":
		status := requestedCode
		if status < 400 || status > 599 {
			status = http.StatusInternalServerError
		}
		http.Error(w, fmt.Sprintf("forced failure (%d)", status), status)
		return true
	default:
		if status, ok := parseFailureStatus(mode); ok {
			http.Error(w, fmt.Sprintf("forced failure (%d)", status), status)
			return true
		}

		http.Error(w, "invalid fail mode", http.StatusBadRequest)
		return true
	}
}

func parseFailureStatus(mode string) (int, bool) {
	status, err := strconv.Atoi(mode)
	if err != nil || status < 400 || status > 599 {
		return 0, false
	}

	return status, true
}

func parseStatusCode(raw string) int {
	if raw == "" {
		return http.StatusOK
	}

	code, err := strconv.Atoi(raw)
	if err != nil || code < 100 || code > 999 {
		return http.StatusOK
	}

	return code
}

func parseSleep(raw string) time.Duration {
	ms, ok := parseMilliseconds(raw, 0)
	if !ok {
		return 0
	}

	return time.Duration(ms) * time.Millisecond
}

func parseTimeout(raw string) time.Duration {
	ms, ok := parseMilliseconds(raw, 5000)
	if !ok {
		return 5 * time.Second
	}
	if ms == 0 {
		return 0
	}

	return time.Duration(ms) * time.Millisecond
}

func parseMilliseconds(raw string, fallback int) (int, bool) {
	if raw == "" {
		return fallback, true
	}

	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 0 {
		return fallback, false
	}

	return ms, true
}

func findNonLoopbackIPv4() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}

		ip := ipNet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}

		return ip.String(), nil
	}

	return "", fmt.Errorf("no non-loopback IPv4 address found; this demo needs one so outbound traffic does not bypass the proxy")
}
