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
	GetDefaultBranchesRequestMessage           MessageType = "GetDefaultBranchesRequest"
	GetDefaultBranchesResponseMessage          MessageType = "GetDefaultBranchesResponse"
)

type Message interface {
	Type() MessageType
}
