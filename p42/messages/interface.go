package messages

type MessageType string

const (
	PingRequestMessage                         MessageType = "PingRequest"
	PingResponseMessage                        MessageType = "PingResponse"
	InvokeAgentRequestMessage                  MessageType = "InvokeAgentRequest"
	InvokeAgentResponseMessage                 MessageType = "InvokeAgentResponse"
	ListOrgsForGithubConnectionRequestMessage  MessageType = "ListOrgsForGithubConnectionRequest"
	ListOrgsForGithubConnectionResponseMessage MessageType = "ListOrgsForGithubConnectionResponse"
	SearchRepoRequestMessage                   MessageType = "SearchRepoRequest"
	SearchRepoResponseMessage                  MessageType = "SearchRepoResponse"
	ListRepoBranchesRequestMessage             MessageType = "ListRepoBranchesRequest"
	ListRepoBranchesResponseMessage            MessageType = "ListRepoBranchesResponse"
	GetPRFeedbackRequestMessage                MessageType = "GetPRFeedbackRequest"
	GetPRFeedbackResponseMessage               MessageType = "GetPRFeedbackResponse"
)

type Message interface {
	Type() MessageType
}
