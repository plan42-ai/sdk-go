package messages

type MessageType string

const (
	PingRequestMessage         MessageType = "PingRequest"
	PingResponseMessage        MessageType = "PingResponse"
	InvokeAgentRequestMessage  MessageType = "InvokeAgentRequest"
	InvokeAgentResponseMessage MessageType = "InvokeAgentResponse"
)

type Message interface {
	Type() MessageType
}
