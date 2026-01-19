package messages

import "encoding/json"

type ListOrgsForGithubConnectionRequest struct {
	TenantID     string
	ConnectionID string
	MaxResults   *int
	Token        *string
	Search       *string
}

func (r *ListOrgsForGithubConnectionRequest) Type() MessageType {
	return ListOrgsForGithubConnectionRequestMessage
}

func (r ListOrgsForGithubConnectionRequest) MarshalJSON() ([]byte, error) {
	var tmp struct {
		ListOrgsForGithubConnectionRequest
		Type MessageType
	}

	tmp.Type = ListOrgsForGithubConnectionRequestMessage
	tmp.ListOrgsForGithubConnectionRequest = r

	return json.Marshal(tmp)
}

type ListOrgsForGithubConnectionResponse struct {
	Items        []string
	NextToken    *string
	ErrorMessage *string
}

func (r *ListOrgsForGithubConnectionResponse) Type() MessageType {
	return ListOrgsForGithubConnectionResponseMessage
}

func (r ListOrgsForGithubConnectionResponse) MarshalJSON() ([]byte, error) {
	var tmp struct {
		ListOrgsForGithubConnectionResponse
		Type MessageType
	}

	tmp.Type = ListOrgsForGithubConnectionResponseMessage
	tmp.ListOrgsForGithubConnectionResponse = r

	return json.Marshal(tmp)
}

type SearchRepoRequest struct {
	TenantID     string
	ConnectionID string
	OrgName      string
	Search       string
	MaxResults   *int
	Token        *string
}

func (r *SearchRepoRequest) Type() MessageType {
	return SearchRepoRequestMessage
}

func (r SearchRepoRequest) MarshalJSON() ([]byte, error) {
	var tmp struct {
		SearchRepoRequest
		Type MessageType
	}

	tmp.Type = SearchRepoRequestMessage
	tmp.SearchRepoRequest = r

	return json.Marshal(tmp)
}

type SearchRepoResponse struct {
	Items        []string
	NextToken    *string
	ErrorMessage *string
}

func (r *SearchRepoResponse) Type() MessageType {
	return SearchRepoResponseMessage
}

func (r SearchRepoResponse) MarshalJSON() ([]byte, error) {
	var tmp struct {
		SearchRepoResponse
		Type MessageType
	}

	tmp.Type = SearchRepoResponseMessage
	tmp.SearchRepoResponse = r

	return json.Marshal(tmp)
}

type ListRepoBranchesRequest struct {
	TenantID     string
	ConnectionID string
	OrgName      string
	RepoName     string
	Search       *string
	MaxResults   *int
	Token        *string
}

func (r *ListRepoBranchesRequest) Type() MessageType {
	return ListRepoBranchesRequestMessage
}

func (r ListRepoBranchesRequest) MarshalJSON() ([]byte, error) {
	var tmp struct {
		ListRepoBranchesRequest
		Type MessageType
	}

	tmp.Type = ListRepoBranchesRequestMessage
	tmp.ListRepoBranchesRequest = r

	return json.Marshal(tmp)
}

type ListRepoBranchesResponse struct {
	Items        []string
	NextToken    *string
	ErrorMessage *string
}

func (r *ListRepoBranchesResponse) Type() MessageType {
	return ListRepoBranchesResponseMessage
}

func (r ListRepoBranchesResponse) MarshalJSON() ([]byte, error) {
	var tmp struct {
		ListRepoBranchesResponse
		Type MessageType
	}

	tmp.Type = ListRepoBranchesResponseMessage
	tmp.ListRepoBranchesResponse = r

	return json.Marshal(tmp)
}
