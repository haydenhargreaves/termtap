package main

// This is a runable example which will spawn two servers, one we can access
// which hits the other and response with the data provided.

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
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
	mux.HandleFunc("/echo", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		message := req.URL.Query().Get("message")
		upstreamURL := fmt.Sprintf("http://%s:3001/echo?message=%s", upstreamHost, url.QueryEscape(message))

		resp, err := http.Get(upstreamURL)
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
	})

	log.Printf("frontend listening on http://127.0.0.1:3000/echo?message=hello")
	log.Printf("frontend calls upstream at http://%s:3001/echo", upstreamHost)
	return http.ListenAndServe("127.0.0.1:3000", mux)
}

func startUpstream(upstreamHost string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		message := req.URL.Query().Get("message")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(message))
	})

	log.Printf("upstream listening on http://%s:3001/echo?message=hello", upstreamHost)
	return http.ListenAndServe(":3001", mux)
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
