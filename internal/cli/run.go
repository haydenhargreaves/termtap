package cli

import (
	"fmt"
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

func Run(args []string) {
	if len(args) >= 2 && args[1] == "cert" {
		runCert()
		return
	}

	cmd, ok := parseCommand(args)
	if !ok {
		displayHelp()
		return
	}

	session, err := app.StartSession(cmd, proxy_addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer session.Stop()

	controls := tui.Controls{
		Restart: session.RestartProcess,
	}

	if err := tui.Run(session.Events, controls); err != nil {
		log.Fatalln(err)
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
	tap cert
	tap run -- <command> [args...]
`

	fmt.Fprintln(os.Stderr, helpText)
}

func runCert() {
	ca, err := proxy.EnsureCertificateAuthority()
	if err != nil {
		log.Fatalln(err)
	}

	certPath := ca.CertPath()
	quotedCertPath := strconv.Quote(certPath)
	fmt.Printf("Certificate path: %s\n", certPath)
	if ca.WasCreated() {
		fmt.Println("Created a new local HTTPS interception CA.")
	} else {
		fmt.Println("Using existing local HTTPS interception CA.")
	}

	trusted, err := ca.IsTrustedBySystem()
	if err != nil {
		fmt.Printf("System trust check failed: %v\n", err)
	} else if trusted {
		fmt.Println("System trust store: trusted")
	} else {
		fmt.Println("System trust store: not trusted")
	}

	if runtime.GOOS != "linux" {
		fmt.Println("Install this certificate into your OS or client trust store to inspect HTTPS traffic.")
		return
	}

	fmt.Println()
	fmt.Println("Trust instructions (Linux):")
	fmt.Println("Debian/Ubuntu:")
	fmt.Printf("  sudo cp %s /usr/local/share/ca-certificates/termtap.crt\n", quotedCertPath)
	fmt.Println("  sudo update-ca-certificates")
	fmt.Println("Fedora/RHEL/CentOS:")
	fmt.Printf("  sudo cp %s /etc/pki/ca-trust/source/anchors/termtap.crt\n", quotedCertPath)
	fmt.Println("  sudo update-ca-trust")
	fmt.Println("Arch:")
	fmt.Printf("  sudo trust anchor %s\n", quotedCertPath)
	fmt.Println()
	fmt.Println("Quick curl test:")
	fmt.Printf("  curl --proxy http://%s --cacert %s https://example.com\n", proxy_addr, quotedCertPath)
}
