package model

import "time"

type EventType string

const (
	EventTypeSessionStarted EventType = "SessionStarted"
	EventTypeSessionStopped EventType = "SessionStopped"

	EventTypeProxyStarting EventType = "ProxyStarting"
	EventTypeProxyStarted  EventType = "ProxyStarted"
	EventTypeProxyStopped  EventType = "ProxyStopped"

	EventTypeRequestStarted  EventType = "RequestStarted"
	EventTypeRequestFinished EventType = "RequestFinished"
	EventTypeRequestFailed   EventType = "RequestFailed"

	EventTypeProcessStarting EventType = "ProcessStarting"
	EventTypeProcessStarted  EventType = "ProcessStarted"
	EventTypeProcessExited   EventType = "ProcessExited"
	EventTypeProcessSignaled EventType = "ProcessSignaled"
	EventTypeProcessStdout   EventType = "ProcessStdout"
	EventTypeProcessStderr   EventType = "ProcessStderr"

	EventTypeFatal EventType = "Fatal"
	EventTypeWarn  EventType = "Warn"
)

type Event struct {
	Time     time.Time
	Type     EventType
	Body     string
	PID      int
	ExitCode int
	Request  Request
}
