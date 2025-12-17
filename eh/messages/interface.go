package messages

type MessageType string

const (
	PingRequestMessage  MessageType = "PingRequest"
	PingResponseMessage MessageType = "PingResponse"
)

type Message interface {
	Type() MessageType
}
