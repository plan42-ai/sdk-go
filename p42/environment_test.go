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
