package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"termtap.dev/internal/app"
	"termtap.dev/internal/model"
	"termtap.dev/internal/tui"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		ok       bool
		nameWant string
		argsWant []string
	}{
		{name: "too few args", args: []string{"tap"}, ok: false},
		{name: "missing run token", args: []string{"tap", "oops", "--", "echo"}, ok: false},
		{name: "missing separator", args: []string{"tap", "run", "echo"}, ok: false},
		{name: "single command", args: []string{"tap", "run", "--", "echo"}, ok: true, nameWant: "echo", argsWant: []string{}},
		{name: "command with args", args: []string{"tap", "run", "--", "curl", "-s", "https://example.com"}, ok: true, nameWant: "curl", argsWant: []string{"-s", "https://example.com"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd, ok := parseCommand(tt.args)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if cmd.Name != tt.nameWant {
				t.Fatalf("cmd.Name = %q, want %q", cmd.Name, tt.nameWant)
			}
			if strings.Join(cmd.Args, "|") != strings.Join(tt.argsWant, "|") {
				t.Fatalf("cmd.Args = %#v, want %#v", cmd.Args, tt.argsWant)
			}
		})
	}
}

func TestDisplayHelpWritesToStderr(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		displayHelp()
	})

	if !strings.Contains(stderr, "tap demo") || !strings.Contains(stderr, "tap cert") || !strings.Contains(stderr, "tap run --") {
		t.Fatalf("stderr missing usage text: %q", stderr)
	}
}

func TestRun_InvalidCommandShowsHelp(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		Run([]string{"tap", "wat"})
	})

	if !strings.Contains(stderr, "usage:") {
		t.Fatalf("stderr missing usage output: %q", stderr)
	}
}

func TestRun_RoutesCertCommand(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	stdout, _ := captureOutput(t, func() {
		Run([]string{"tap", "cert"})
	})

	if !strings.Contains(stdout, "Certificate path:") {
		t.Fatalf("stdout missing certificate path output: %q", stdout)
	}
}

func TestRun_RoutesDemoCommand(t *testing.T) {
	restore := installRunSeams(t)
	defer restore()

	called := installFatalSpy(t)
	seen := 0
	runTUIFn = func(ch <-chan model.Event, _ tui.Controls) error {
		for i := 0; i < 3; i++ {
			select {
			case ev := <-ch:
				seen++
				if seen == 1 && ev.Type != model.EventTypeSessionStarted {
					t.Fatalf("first demo event = %s, want %s", ev.Type, model.EventTypeSessionStarted)
				}
			default:
				return nil
			}
		}
		return nil
	}

	Run([]string{"tap", "demo"})
	if *called {
		t.Fatal("fatalExit should not be called for demo command")
	}
	if seen == 0 {
		t.Fatal("expected demo event stream to be seeded")
	}
}

func TestRun_StartSessionFailureCallsFatalExit(t *testing.T) {
	restore := installRunSeams(t)
	defer restore()

	startSessionFn = func(model.Command, string) (*app.Session, error) {
		return nil, errors.New("boom")
	}
	called := installFatalSpy(t)

	Run([]string{"tap", "run", "--", "definitely-not-a-real-command"})
	if !*called {
		t.Fatal("expected fatalExit to be called on StartSession failure")
	}
}

func TestRun_TUIFailureCallsFatalExit(t *testing.T) {
	restore := installRunSeams(t)
	defer restore()

	startSessionFn = func(model.Command, string) (*app.Session, error) {
		return &app.Session{Events: make(chan model.Event)}, nil
	}
	runTUIFn = func(<-chan model.Event, tui.Controls) error {
		return errors.New("tui failed")
	}
	called := installFatalSpy(t)

	Run([]string{"tap", "run", "--", "echo"})
	if !*called {
		t.Fatal("expected fatalExit to be called on tui failure")
	}
}

func TestRun_SuccessPathDoesNotCallFatal(t *testing.T) {
	restore := installRunSeams(t)
	defer restore()

	startSessionFn = func(model.Command, string) (*app.Session, error) {
		return &app.Session{Events: make(chan model.Event)}, nil
	}
	runTUIFn = func(<-chan model.Event, tui.Controls) error {
		return nil
	}
	called := installFatalSpy(t)

	Run([]string{"tap", "run", "--", "echo"})
	if *called {
		t.Fatal("fatalExit should not be called on success path")
	}
}

func TestRunCert_EnsureCAFailureCallsFatalExit(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	called := installFatalSpy(t)

	runCert()
	if !*called {
		t.Fatal("expected fatalExit to be called when EnsureCertificateAuthority fails")
	}
}

func TestRunCertOutputContract(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	stdout, _ := captureOutput(t, func() {
		runCert()
	})

	if !strings.Contains(stdout, "Certificate path:") {
		t.Fatalf("stdout missing certificate path line: %q", stdout)
	}
	if !strings.Contains(stdout, "local HTTPS interception CA") {
		t.Fatalf("stdout missing CA create/existing line: %q", stdout)
	}
	if !strings.Contains(stdout, "System trust store:") && !strings.Contains(stdout, "System trust check failed:") {
		t.Fatalf("stdout missing trust check line: %q", stdout)
	}

	if runtime.GOOS == "linux" {
		if !strings.Contains(stdout, "Trust instructions (Linux):") {
			t.Fatalf("stdout missing linux trust instructions: %q", stdout)
		}
	}
}

func TestRunCert_CreatedThenExistingMessage(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	firstOut, _ := captureOutput(t, func() {
		runCert()
	})
	if !strings.Contains(firstOut, "Created a new local HTTPS interception CA.") {
		t.Fatalf("first run should indicate created CA, got: %q", firstOut)
	}

	secondOut, _ := captureOutput(t, func() {
		runCert()
	})
	if !strings.Contains(secondOut, "Using existing local HTTPS interception CA.") {
		t.Fatalf("second run should indicate existing CA, got: %q", secondOut)
	}
}

func captureOutput(t *testing.T, fn func()) (stdout string, stderr string) {
	t.Helper()

	origStdoutWriter := stdoutWriter
	origStderrWriter := stderrWriter
	t.Cleanup(func() {
		stdoutWriter = origStdoutWriter
		stderrWriter = origStderrWriter
	})

	origStdout := os.Stdout
	origStderr := os.Stderr

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stdout = outW
	os.Stderr = errW
	stdoutWriter = outW
	stderrWriter = errW

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, outR)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, errR)
		errCh <- buf.String()
	}()

	fn()

	_ = outW.Close()
	_ = errW.Close()
	stdoutWriter = origStdoutWriter
	stderrWriter = origStderrWriter
	os.Stdout = origStdout
	os.Stderr = origStderr

	select {
	case stdout = <-outCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stdout capture")
	}
	select {
	case stderr = <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stderr capture")
	}
	_ = outR.Close()
	_ = errR.Close()

	return stdout, stderr
}

func TestDisplayHelp_UsesInjectedStderrWriter(t *testing.T) {
	var buf bytes.Buffer
	orig := stderrWriter
	t.Cleanup(func() { stderrWriter = orig })
	stderrWriter = &buf

	displayHelp()

	if got := buf.String(); !strings.Contains(got, "usage:") {
		t.Fatalf("help output missing usage, got: %q", got)
	}
}

func TestRunCert_UsesInjectedStdoutWriter(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	var buf bytes.Buffer
	orig := stdoutWriter
	t.Cleanup(func() { stdoutWriter = orig })
	stdoutWriter = &buf

	runCert()

	if got := buf.String(); !strings.Contains(got, "Certificate path:") {
		t.Fatalf("cert output missing path line, got: %q", got)
	}
}

func installRunSeams(t *testing.T) func() {
	t.Helper()

	origStartSession := startSessionFn
	origRunTUI := runTUIFn
	return func() {
		startSessionFn = origStartSession
		runTUIFn = origRunTUI
	}
}

func installFatalSpy(t *testing.T) *bool {
	t.Helper()

	origFatal := fatalExit
	called := false
	fatalExit = func(v ...any) {
		called = true
	}
	t.Cleanup(func() {
		fatalExit = origFatal
	})

	return &called
}

func TestStdioRefWrite(t *testing.T) {
	t.Run("writes to stdout", func(t *testing.T) {
		assertStdioRefWrite(t, false, "hello")
	})

	t.Run("writes to stderr", func(t *testing.T) {
		assertStdioRefWrite(t, true, "boom")
	})
}

func assertStdioRefWrite(t *testing.T, isErr bool, payload string) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	defer func() { _ = r.Close() }()

	if isErr {
		orig := os.Stderr
		os.Stderr = w
		defer func() { os.Stderr = orig }()
	} else {
		orig := os.Stdout
		os.Stdout = w
		defer func() { os.Stdout = orig }()
	}

	if _, err := (stdioRef{isErr: isErr}).Write([]byte(payload)); err != nil {
		_ = w.Close()
		t.Fatalf("stdioRef write error: %v", err)
	}
	_ = w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(pipe) error: %v", err)
	}
	if got := string(data); got != payload {
		t.Fatalf("pipe write = %q, want %q", got, payload)
	}
}
