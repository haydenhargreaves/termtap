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

const defaultProxyPort = 8080

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

	cmd, proxyAddr, ok := parseCommand(args)
	if !ok {
		displayHelp()
		return
	}

	session, err := startSessionFn(cmd, proxyAddr)
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

func parseCommand(args []string) (model.Command, string, bool) {
	if len(args) < 4 {
		return model.Command{}, "", false
	}

	if args[1] != "run" {
		return model.Command{}, "", false
	}

	port := defaultProxyPort
	idx := 2
	for idx < len(args) && args[idx] != "--" {
		if args[idx] != "--port" {
			return model.Command{}, "", false
		}

		if idx+1 >= len(args) {
			return model.Command{}, "", false
		}

		p, err := strconv.Atoi(args[idx+1])
		if err != nil || p <= 0 || p > 65535 {
			return model.Command{}, "", false
		}

		port = p
		idx += 2
	}

	if idx >= len(args) || args[idx] != "--" {
		return model.Command{}, "", false
	}

	if idx+1 >= len(args) {
		return model.Command{}, "", false
	}

	cmdArgs := args[idx+1:]
	cmd := model.Command{Name: cmdArgs[0], Args: cmdArgs[1:]}
	return cmd, proxyAddrForPort(port), true
}

func proxyAddrForPort(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func displayHelp() {
	helpText := `
usage:
	tap demo
	tap cert
	tap run [--port <port>] -- <command> [args...]
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
	fmt.Fprintf(stdoutWriter, "  curl --proxy http://%s --cacert %s https://example.com\n", proxyAddrForPort(defaultProxyPort), quotedCertPath)
}
