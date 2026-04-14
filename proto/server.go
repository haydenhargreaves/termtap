package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	if err := startDemoServer("127.0.0.1:3000"); err != nil {
		panic(err)
	}
}

func startDemoServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		resp, err := http.Get("http://example.com")
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

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "sent request to http://example.com\nstatus: %s\nbytes: %d\n", resp.Status, len(body))
	})

	fmt.Printf("demo server listening on http://%s/send\n", addr)
	return http.ListenAndServe(addr, mux)
}
