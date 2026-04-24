package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain_SmokeInvokesCLIRun(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"tap", "invalid"}

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = r.Close()
	})
	os.Stderr = w

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	main()

	_ = w.Close()
	os.Stderr = origStderr
	var got string
	select {
	case got = <-outCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stderr capture")
	}

	if !strings.Contains(got, "usage:") {
		t.Fatalf("stderr missing usage output, got: %q", got)
	}
}
