package eh

import "time"

// EnvVar defines an environment variable for an Environment.
type EnvVar struct {
	Name     string `json:"Name"`
	Value    string `json:"Value"`
	IsSecret bool   `json:"IsSecret"`
}

// Environment represents an execution environment.
type Environment struct {
	TenantID      string    `json:"TenantId"`
	EnvironmentID string    `json:"EnvironmentId"`
	Name          string    `json:"Name"`
	Description   string    `json:"Description"`
	Context       string    `json:"Context"`
	Repos         []string  `json:"Repos"`
	SetupScript   string    `json:"SetupScript"`
	DockerImage   string    `json:"DockerImage"`
	AllowedHosts  []string  `json:"AllowedHosts"`
	EnvVars       []EnvVar  `json:"EnvVars"`
	CreatedAt     time.Time `json:"CreatedAt"`
	UpdatedAt     time.Time `json:"UpdatedAt"`
}

// ObjectType returns the object type for ConflictError handling.
func (Environment) ObjectType() ObjectType { return ObjectTypeEnvironment }
