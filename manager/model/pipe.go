package model

const (
	TopicSeperator       = ":"
	OperationSubscribe   = "SUB"
	OperationUnsubscribe = "UNSUB"
	PipeConnected        = "CONN"
	PipeDisconnected     = "DISCONN"
)

// Pipe is a bi-directional channel structure
//	for communication between the clients and manager/agent
type Pipe struct {
	RequestCh   chan Message
	ResponseCh  chan Message
	OperationCh chan Message
}

// NewPipe returns an instantiated Pipe
func NewPipe() Pipe {
	return Pipe{
		RequestCh:   make(chan Message),
		ResponseCh:  make(chan Message),
		OperationCh: make(chan Message),
	}
}

type Message struct {
	Topic   string
	Payload []byte
}
