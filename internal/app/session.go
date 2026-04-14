package app

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"termtap.dev/internal/model"
)

func StartSession(cmd model.Command, addr string) error {

	// Event type?
	msgs := make(chan model.Message, 128)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Start process and proxy
	go StartProxy(addr, msgs)
	go StartProcess(cmd, addr, msgs, sigCh)

	var events []model.Message

	for {
		select {
		case _ = <-sigCh:
			printEvents(events)
			return nil
		case msg := <-msgs:
			{
				events = append(events, msg)
				switch msg.Type {
				case model.MessageTypeFatal:
					return fmt.Errorf("%s", msg.Body)
				default:
					log.Printf("[%s] %s", msg.Type, msg.Body)
				}
			}
		}
	}
}

// DEBUG
func printEvents(events []model.Message) {
	for _, event := range events {
		fmt.Printf("%+v\n", event)
	}
}
