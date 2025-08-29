package eh

import "time"

// GithubOrg represents a github organization.
type GithubOrg struct {
	OrgID          string    `json:"OrgID"`
	OrgName        string    `json:"OrgName"`
	ExternalOrgID  int       `json:"ExternalOrgID"`
	InstallationID int       `json:"InstallationID"`
	CreatedAt      time.Time `json:"CreatedAt"`
	UpdatedAt      time.Time `json:"UpdatedAt"`
	Version        int       `json:"Version"`
	Deleted        bool      `json:"Deleted"`
}

// ObjectType returns the object type for ConflictError handling.
func (GithubOrg) ObjectType() ObjectType { return ObjectTypeGithubOrg }

// TenantGithubOrg represents the association between a tenant and a github organization.
type TenantGithubOrg struct {
	TenantID          string    `json:"TenantID"`
	OrgID             string    `json:"OrgID"`
	GithubUserID      int       `json:"GithubUserID"`
	OAuthToken        string    `json:"OAuthToken"`
	OAuthRefreshToken string    `json:"OAuthRefreshToken"`
	ExpiresAt         time.Time `json:"ExpiresAt"`
	OrgName           string    `json:"OrgName"`
	InstallationID    int       `json:"InstallationID"`
	ExternalOrgID     int       `json:"ExternalOrgID"`
	CreatedAt         time.Time `json:"CreatedAt"`
	UpdatedAt         time.Time `json:"UpdatedAt"`
	Version           int       `json:"Version"`
	Deleted           bool      `json:"Deleted"`
}

// ObjectType returns the object type for ConflictError handling.
func (TenantGithubOrg) ObjectType() ObjectType { return ObjectTypeTenantGithubOrg }
