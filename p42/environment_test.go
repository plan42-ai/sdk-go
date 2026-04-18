package p42

import (
	"encoding/json"
	"testing"
)

func TestEnvironmentMarshalDefaults(t *testing.T) {
	env := Environment{}

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("failed to marshal environment: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal encoded environment: %v", err)
	}

	if decoded["RunnerId"] != environmentDefaultID {
		t.Fatalf("expected RunnerId to default, got %v", decoded["RunnerId"])
	}
	if decoded["GithubConnectionId"] != environmentDefaultID {
		t.Fatalf("expected GithubConnectionId to default, got %v", decoded["GithubConnectionId"])
	}
}

func TestEnvironmentUnmarshalDefaults(t *testing.T) {
	input := `{"TenantId":"t","EnvironmentId":"e","Name":"","Description":"","Context":"","Repos":[],"SetupScript":"","DockerImage":"","AllowedHosts":[],"EnvVars":[],"Deleted":false,"Version":1}`

	var env Environment
	if err := json.Unmarshal([]byte(input), &env); err != nil {
		t.Fatalf("failed to unmarshal environment: %v", err)
	}

	if env.RunnerID == nil || *env.RunnerID != environmentDefaultID {
		t.Fatalf("expected RunnerID to default, got %v", env.RunnerID)
	}
	if env.GithubConnectionID == nil || *env.GithubConnectionID != environmentDefaultID {
		t.Fatalf("expected GithubConnectionID to default, got %v", env.GithubConnectionID)
	}
}

func TestEnvironmentRedactSecrets(t *testing.T) {
	env := &Environment{
		EnvVars: []EnvVar{
			{Name: "VISIBLE", Value: "public", IsSecret: false},
			{Name: "SECRET", Value: "top-secret", IsSecret: true},
			{Name: "EMPTY_SECRET", Value: "", IsSecret: true},
		},
	}

	env.RedactSecrets()

	if got := env.EnvVars[0]; got.Value != "public" || got.IsSecret {
		t.Fatalf("expected non-secret env var to be preserved, got %+v", got)
	}
	if got := env.EnvVars[1]; got.Value != "" || !got.IsSecret || got.Name != "SECRET" {
		t.Fatalf("expected secret env var to be redacted in place, got %+v", got)
	}
	if got := env.EnvVars[2]; got.Value != "" || !got.IsSecret || got.Name != "EMPTY_SECRET" {
		t.Fatalf("expected empty secret env var metadata to be preserved, got %+v", got)
	}
}

func TestEnvironmentRedactSecretsNilReceiver(_ *testing.T) {
	var env *Environment
	env.RedactSecrets()
}
