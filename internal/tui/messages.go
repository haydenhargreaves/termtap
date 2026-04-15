package tui

import "termtap.dev/internal/model"

type EventMsg struct {
	value model.Event
}

type ErrMsg struct {
	err error
}
