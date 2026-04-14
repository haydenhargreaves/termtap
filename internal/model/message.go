package model

type MessageType string

const (
	MessageTypeSessionStarted MessageType = "SessionStarted"
	MessageTypeSessionStopped MessageType = "SessionStopped"

	MessageTypeProxyStarting MessageType = "ProxyStarting"
	MessageTypeProxyStarted  MessageType = "ProxyStarted"
	MessageTypeProxyStopped  MessageType = "ProxyStopped"

	MessageTypeRequestStarted  MessageType = "RequestStarted"
	MessageTypeRequestFinished MessageType = "RequestFinished"
	MessageTypeRequestFailed   MessageType = "RequestFailed"

	MessageTypeProcessStarting MessageType = "ProcessStarting"
	MessageTypeProcessStarted  MessageType = "ProcessStarted"
	MessageTypeProcessExited   MessageType = "ProcessExited"
	MessageTypeProcessSignaled MessageType = "ProcessSignaled"
	MessageTypeProcessStdout   MessageType = "ProcessStdout"
	MessageTypeProcessStderr   MessageType = "ProcessStderr"

	MessageTypeFatal MessageType = "Fatal"
	MessageTypeWarn  MessageType = "Warn"
)

type Message struct {
	Type      MessageType
	Body      string
	PID       int
	RequestID string
	URL       string
	Status    int
	ExitCode  int
}
