package messages

import "encoding/json"

type ListOrgsForGithubConnectionRequest struct {
	TenantID     string
	ConnectionID string
	MaxResults   *int
	Token        *string
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
	}

	tmp.Type = ListOrgsForGithubConnectionRequestMessage
	tmp.TenantID = r.TenantID
	tmp.ConnectionID = r.ConnectionID
	tmp.MaxResults = r.MaxResults
	tmp.Token = r.Token

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
