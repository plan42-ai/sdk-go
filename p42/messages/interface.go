package messages

type MessageType string

const (
	PingRequestMessage                         MessageType = "PingRequest"
	PingResponseMessage                        MessageType = "PingResponse"
	InvokeAgentRequestMessage                  MessageType = "InvokeAgentRequest"
	InvokeAgentResponseMessage                 MessageType = "InvokeAgentResponse"
	ListOrgsForGithubConnectionRequestMessage  MessageType = "ListOrgsForGithubConnectionRequest"
	ListOrgsForGithubConnectionResponseMessage MessageType = "ListOrgsForGithubConnectionResponse"
)

type Message interface {
	Type() MessageType
}
