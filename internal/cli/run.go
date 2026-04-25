package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strconv"

	"termtap.dev/internal/app"
	"termtap.dev/internal/model"
	"termtap.dev/internal/proxy"
	"termtap.dev/internal/tui"
)

// This should be configurable at some point, just in case they build on 8080
const proxy_addr = "127.0.0.1:8080"

var fatalExit = log.Fatalln
var stdoutWriter io.Writer = stdioRef{isErr: false}
var stderrWriter io.Writer = stdioRef{isErr: true}
var startSessionFn = app.StartSession
var runTUIFn = tui.Run

type stdioRef struct {
	isErr bool
}

func (w stdioRef) Write(p []byte) (int, error) {
	if w.isErr {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func Run(args []string) {
	if len(args) >= 2 && args[1] == "cert" {
		runCert()
		return
	}
	if len(args) >= 2 && args[1] == "demo" {
		runDemo()
		return
	}

	cmd, ok := parseCommand(args)
	if !ok {
		displayHelp()
		return
	}

	session, err := startSessionFn(cmd, proxy_addr)
	if err != nil {
		fatalExit(err)
		return
	}
	defer session.Stop()

	controls := tui.Controls{
		Restart: session.RestartProcess,
	}

	if err := runTUIFn(session.Events, controls); err != nil {
		fatalExit(err)
		return
	}
}

func parseCommand(args []string) (model.Command, bool) {
	if len(args) < 4 {
		return model.Command{}, false
	}

	if args[1] != "run" || args[2] != "--" {
		return model.Command{}, false
	}

	args = args[3:]
	if len(args) == 1 {
		return model.Command{Name: args[0], Args: []string{}}, true
	}

	return model.Command{Name: args[0], Args: args[1:]}, true
}

func displayHelp() {
	helpText := `
usage:
	tap demo
	tap cert
	tap run -- <command> [args...]
`

	fmt.Fprintln(stderrWriter, helpText)
}

func runCert() {
	ca, err := proxy.EnsureCertificateAuthority()
	if err != nil {
		fatalExit(err)
		return
	}

	certPath := ca.CertPath()
	quotedCertPath := strconv.Quote(certPath)
	fmt.Fprintf(stdoutWriter, "Certificate path: %s\n", certPath)
	if ca.WasCreated() {
		fmt.Fprintln(stdoutWriter, "Created a new local HTTPS interception CA.")
	} else {
		fmt.Fprintln(stdoutWriter, "Using existing local HTTPS interception CA.")
	}

	trusted, err := ca.IsTrustedBySystem()
	if err != nil {
		fmt.Fprintf(stdoutWriter, "System trust check failed: %v\n", err)
	} else if trusted {
		fmt.Fprintln(stdoutWriter, "System trust store: trusted")
	} else {
		fmt.Fprintln(stdoutWriter, "System trust store: not trusted")
	}

	if runtime.GOOS != "linux" {
		fmt.Fprintln(stdoutWriter, "Install this certificate into your OS or client trust store to inspect HTTPS traffic.")
		return
	}

	fmt.Fprintln(stdoutWriter)
	fmt.Fprintln(stdoutWriter, "Trust instructions (Linux):")
	fmt.Fprintln(stdoutWriter, "Debian/Ubuntu:")
	fmt.Fprintf(stdoutWriter, "  sudo cp %s /usr/local/share/ca-certificates/termtap.crt\n", quotedCertPath)
	fmt.Fprintln(stdoutWriter, "  sudo update-ca-certificates")
	fmt.Fprintln(stdoutWriter, "Fedora/RHEL/CentOS:")
	fmt.Fprintf(stdoutWriter, "  sudo cp %s /etc/pki/ca-trust/source/anchors/termtap.crt\n", quotedCertPath)
	fmt.Fprintln(stdoutWriter, "  sudo update-ca-trust")
	fmt.Fprintln(stdoutWriter, "Arch:")
	fmt.Fprintf(stdoutWriter, "  sudo trust anchor %s\n", quotedCertPath)
	fmt.Fprintln(stdoutWriter)
	fmt.Fprintln(stdoutWriter, "Quick curl test:")
	fmt.Fprintf(stdoutWriter, "  curl --proxy http://%s --cacert %s https://example.com\n", proxy_addr, quotedCertPath)
}
