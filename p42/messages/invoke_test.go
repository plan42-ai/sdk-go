package messages_test

import (
	"encoding/json"
	"testing"

	"github.com/plan42-ai/sdk-go/p42"
	"github.com/plan42-ai/sdk-go/p42/messages"
	"github.com/stretchr/testify/require"
)

func TestInvokeAgentRequestRoundTripSubAgent(t *testing.T) {
	t.Parallel()

	openAIEndpoint := "https://openai.example.test/v1"
	openAIToken := "openai-token"
	claudeEndpoint := "https://claude.example.test"
	claudeToken := "claude-token"
	req := messages.InvokeAgentRequest{
		OpenAIEndpoint: &openAIEndpoint,
		OpenAIToken:    &openAIToken,
		ClaudeEndpoint: &claudeEndpoint,
		ClaudeToken:    &claudeToken,
		SubAgent: &p42.SubAgent{
			AgentID:        "agent-1",
			AgentType:      p42.AgentTypeCustom,
			Name:           "Reviewer",
			Model:          p42.ModelTypeClaude48Opus,
			ReasoningLevel: p42.ReasoningLevelHigh,
			Status:         p42.AgentStatusRunning,
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Contains(t, raw, "SubAgent")
	require.Contains(t, raw, "OpenAIEndpoint")
	require.Contains(t, raw, "OpenAIToken")
	require.Contains(t, raw, "ClaudeEndpoint")
	require.Contains(t, raw, "ClaudeToken")

	var got messages.InvokeAgentRequest
	require.NoError(t, json.Unmarshal(data, &got))
	require.NotNil(t, got.OpenAIEndpoint)
	require.Equal(t, openAIEndpoint, *got.OpenAIEndpoint)
	require.NotNil(t, got.OpenAIToken)
	require.Equal(t, openAIToken, *got.OpenAIToken)
	require.NotNil(t, got.ClaudeEndpoint)
	require.Equal(t, claudeEndpoint, *got.ClaudeEndpoint)
	require.NotNil(t, got.ClaudeToken)
	require.Equal(t, claudeToken, *got.ClaudeToken)
	require.NotNil(t, got.SubAgent)
	require.Equal(t, req.SubAgent.AgentID, got.SubAgent.AgentID)
	require.Equal(t, req.SubAgent.AgentType, got.SubAgent.AgentType)
	require.Equal(t, req.SubAgent.Model, got.SubAgent.Model)
	require.Equal(t, req.SubAgent.ReasoningLevel, got.SubAgent.ReasoningLevel)
	require.Equal(t, req.SubAgent.Status, got.SubAgent.Status)
}

func TestInvokeAgentRequestGetModelPrefersSubAgent(t *testing.T) {
	t.Parallel()

	taskModel := p42.ModelTypeGpt54
	req := messages.InvokeAgentRequest{
		Task: &p42.Task{Model: &taskModel},
		SubAgent: &p42.SubAgent{
			Model: p42.ModelTypeClaude48Opus,
		},
	}

	require.Equal(t, p42.ModelTypeClaude48Opus, req.GetModel())
}

func TestInvokeAgentRequestGetModelFallbacks(t *testing.T) {
	t.Parallel()

	taskModel := p42.ModelTypeGpt53Codex
	require.Equal(
		t,
		taskModel,
		(&messages.InvokeAgentRequest{Task: &p42.Task{Model: &taskModel}}).GetModel(),
	)
	require.Equal(t, p42.ModelTypeGpt51Codex, (&messages.InvokeAgentRequest{}).GetModel())
	require.Equal(t, p42.ModelTypeGpt51Codex, (*messages.InvokeAgentRequest)(nil).GetModel())
}
