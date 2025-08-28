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
