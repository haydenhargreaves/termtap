package cli

import (
	"fmt"
	"log"
	"os"

	"termtap.dev/internal/app"
	"termtap.dev/internal/model"
	"termtap.dev/internal/tui"
)

// This should be configurable at some point, just in case they build on 8080
const proxy_addr = "127.0.0.1:8080"

func Run(args []string) {
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
	tap run -- <command> [args...]
`

	fmt.Fprintln(os.Stderr, helpText)
}
