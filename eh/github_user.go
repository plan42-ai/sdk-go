package eh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// User represents a user in Event Horizon.
// The fields included are those returned by the API sections relevant to
// GitHub credentials. Additional fields can be added in the future without
// breaking backwards-compatibility.
type User struct {
	TenantID        string     `json:"TenantID"`
	FullName        *string    `json:"FullName,omitempty"`
	FirstName       *string    `json:"FirstName,omitempty"`
	LastName        *string    `json:"LastName,omitempty"`
	Email           *string    `json:"Email,omitempty"`
	PictureURL      *string    `json:"PictureUrl,omitempty"`
	GithubUserLogin *string    `json:"GithubUserLogin,omitempty"`
	GithubUserID    *int       `json:"GithubUserID,omitempty"`
	CreatedAt       *time.Time `json:"CreatedAt,omitempty"`
	UpdatedAt       *time.Time `json:"UpdatedAt,omitempty"`
	Version         *int       `json:"Version,omitempty"`
	Deleted         *bool      `json:"Deleted,omitempty"`
}

// FindGithubUserRequest is the request for FindGithubUser.
// Exactly one of GithubID or GithubLogin must be provided.
type FindGithubUserRequest struct {
	GithubID    *int
	GithubLogin *string
	MaxResults  *int
	Token       *string
}

// GetField retrieves the value of a field by name. This is primarily used by
// test helpers to perform golden-file style comparisons.
// nolint:goconst
func (r *FindGithubUserRequest) GetField(name string) (any, bool) {
	switch name {
	case "GithubID":
		return evalNullable(r.GithubID)
	case "GithubLogin":
		return evalNullable(r.GithubLogin)
	case "MaxResults":
		return evalNullable(r.MaxResults)
	case "Token":
		return evalNullable(r.Token)
	default:
		return nil, false
	}
}

// FindGithubUserResponse is the response from FindGithubUser.
type FindGithubUserResponse struct {
	Users     []User  `json:"Users"`
	NextToken *string `json:"NextToken"`
}

// FindGithubUser looks up users by their GitHub login or ID.
//
// Exactly one of githubID or githubLogin must be provided as per the API
// specification. If both are provided or neither is provided an error is
// returned before any HTTP request is executed.
func (c *Client) FindGithubUser(ctx context.Context, req *FindGithubUserRequest) (*FindGithubUserResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("req is nil")
	}

	// Validate exclusivity of search parameters.
	idProvided := req.GithubID != nil
	loginProvided := req.GithubLogin != nil
	if idProvided == loginProvided { // both true or both false
		return nil, fmt.Errorf("exactly one of githubID or githubLogin must be provided")
	}

	u := c.BaseURL.JoinPath("v1", "users")
	q := u.Query()

	if idProvided {
		q.Set("githubID", strconv.Itoa(*req.GithubID))
	}
	if loginProvided {
		q.Set("githubLogin", *req.GithubLogin)
	}
	if req.MaxResults != nil {
		q.Set("maxResults", strconv.Itoa(*req.MaxResults))
	}
	if req.Token != nil {
		q.Set("token", *req.Token)
	}

	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")

	if err := c.authenticate(DelegatedAuthInfo{}, httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}

	var out FindGithubUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
