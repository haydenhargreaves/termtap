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

	var requests []model.Request

	for {
		select {
		case _ = <-sigCh:
			fmt.Println("\n\nEVENTS")
			printEvents(events)
			fmt.Println("\n\nREQUESTS")
			printRequests(requests)
			return nil
		case msg := <-msgs:
			{
				events = append(events, msg)
				switch msg.Type {
				case model.MessageTypeFatal:
					return fmt.Errorf("%s", msg.Body)

				case model.MessageTypeRequestStarted:
					log.Printf("[%s] (%s) %s", msg.Type, msg.Request.ID.String(), msg.Body)
					requests = append(requests, msg.Request)

				case model.MessageTypeRequestFinished, model.MessageTypeRequestFailed:
					log.Printf("[%s] (%s) %s", msg.Type, msg.Request.ID.String(), msg.Body)

					for i := range requests {
						if requests[i].ID == msg.Request.ID {
							requests[i] = msg.Request
							break
						}
					}

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

func printRequests(reqs []model.Request) {
	for _, req := range reqs {
		fmt.Printf("%+v\n", req)
		for k, v := range req.QueryMap {
			fmt.Printf("key: %s, vals: %+v\n", k, v)
		}
	}
}
