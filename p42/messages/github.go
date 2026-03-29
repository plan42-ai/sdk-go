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
		Type         MessageType
		TenantID     string
		ConnectionID string
		MaxResults   *int
		Token        *string
		Search       *string
	}

	tmp.Type = ListOrgsForGithubConnectionRequestMessage
	tmp.TenantID = r.TenantID
	tmp.ConnectionID = r.ConnectionID
	tmp.MaxResults = r.MaxResults
	tmp.Token = r.Token
	tmp.Search = r.Search

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
		Type         MessageType
		Items        []string
		NextToken    *string
		ErrorMessage *string
	}

	tmp.Type = ListOrgsForGithubConnectionResponseMessage
	tmp.Items = r.Items
	tmp.NextToken = r.NextToken
	tmp.ErrorMessage = r.ErrorMessage

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
		Type         MessageType
		TenantID     string
		ConnectionID string
		OrgName      string
		Search       string
		MaxResults   *int
		Token        *string
	}

	tmp.Type = SearchRepoRequestMessage
	tmp.TenantID = r.TenantID
	tmp.ConnectionID = r.ConnectionID
	tmp.OrgName = r.OrgName
	tmp.Search = r.Search
	tmp.MaxResults = r.MaxResults
	tmp.Token = r.Token

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
		Type         MessageType
		Items        []string
		NextToken    *string
		ErrorMessage *string
	}

	tmp.Type = SearchRepoResponseMessage
	tmp.Items = r.Items
	tmp.NextToken = r.NextToken
	tmp.ErrorMessage = r.ErrorMessage

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
		Type         MessageType
		TenantID     string
		ConnectionID string
		OrgName      string
		RepoName     string
		Search       *string
		MaxResults   *int
		Token        *string
	}

	tmp.Type = ListRepoBranchesRequestMessage
	tmp.TenantID = r.TenantID
	tmp.ConnectionID = r.ConnectionID
	tmp.OrgName = r.OrgName
	tmp.RepoName = r.RepoName
	tmp.Search = r.Search
	tmp.MaxResults = r.MaxResults
	tmp.Token = r.Token

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
		Type         MessageType
		Items        []string
		NextToken    *string
		ErrorMessage *string
	}

	tmp.Type = ListRepoBranchesResponseMessage
	tmp.Items = r.Items
	tmp.NextToken = r.NextToken
	tmp.ErrorMessage = r.ErrorMessage

	return json.Marshal(tmp)
}

// RepoBranch represents the default branch for a single org/repo.
type RepoBranch struct {
	Repo          string `json:"Repo"`
	DefaultBranch string `json:"DefaultBranch"`
}

type GetDefaultBranchesRequest struct {
	TenantID     string
	ConnectionID string
	Repos        []string
}

func (r *GetDefaultBranchesRequest) Type() MessageType {
	return GetDefaultBranchesRequestMessage
}

func (r GetDefaultBranchesRequest) MarshalJSON() ([]byte, error) {
	var tmp struct {
		Type         MessageType
		TenantID     string
		ConnectionID string
		Repos        []string
	}

	tmp.Type = GetDefaultBranchesRequestMessage
	tmp.TenantID = r.TenantID
	tmp.ConnectionID = r.ConnectionID
	tmp.Repos = r.Repos

	return json.Marshal(tmp)
}

type GetDefaultBranchesResponse struct {
	Items        []RepoBranch
	ErrorMessage *string
}

func (r *GetDefaultBranchesResponse) Type() MessageType {
	return GetDefaultBranchesResponseMessage
}

func (r GetDefaultBranchesResponse) MarshalJSON() ([]byte, error) {
	var tmp struct {
		Type         MessageType
		Items        []RepoBranch
		ErrorMessage *string
	}

	tmp.Type = GetDefaultBranchesResponseMessage
	tmp.Items = r.Items
	tmp.ErrorMessage = r.ErrorMessage

	return json.Marshal(tmp)
}
