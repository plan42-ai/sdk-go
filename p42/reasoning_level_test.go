package p42

import (
	"slices"
	"testing"
)

func TestSupportedReasoningLevels(t *testing.T) {
	t.Parallel()

	full := []ReasoningLevel{ReasoningLevelLow, ReasoningLevelMedium, ReasoningLevelHigh, ReasoningLevelMax}
	noMax := []ReasoningLevel{ReasoningLevelLow, ReasoningLevelMedium, ReasoningLevelHigh}

	cases := []struct {
		model ModelType
		want  []ReasoningLevel
	}{
		{ModelTypeClaude45Opus, noMax},
		{ModelTypeGpt51Codex, noMax},
		{ModelTypeClaude46Opus, full},
		{ModelTypeClaude48Opus, full},
		{ModelTypeGpt51CodexMax, full},
		{ModelTypeChatGpt55, full},
		// Unknown/future models default to the full range (no per-model change needed).
		{ModelType("Some Future Model"), full},
	}

	for _, tc := range cases {
		if got := tc.model.SupportedReasoningLevels(); !slices.Equal(got, tc.want) {
			t.Errorf("%q: SupportedReasoningLevels() = %v, want %v", tc.model, got, tc.want)
		}
	}

	if ModelTypeClaude45Opus.SupportsReasoningLevel(ReasoningLevelMax) {
		t.Error("Claude 4.5 Opus should not support Max")
	}
	if !ModelTypeClaude46Opus.SupportsReasoningLevel(ReasoningLevelMax) {
		t.Error("Claude 4.6 Opus should support Max")
	}
}

func TestNormalizeReasoningLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   ReasoningLevel
		want    ReasoningLevel
		wantErr bool
	}{
		{name: "canonical low", input: "Low", want: ReasoningLevelLow},
		{name: "canonical medium", input: "Medium", want: ReasoningLevelMedium},
		{name: "canonical high", input: "High", want: ReasoningLevelHigh},
		{name: "canonical max", input: "Max", want: ReasoningLevelMax},
		{name: "lowercase max", input: "max", want: ReasoningLevelMax},
		{name: "uppercase max", input: "MAX", want: ReasoningLevelMax},
		{name: "mixed case medium", input: "mEdIuM", want: ReasoningLevelMedium},
		{name: "lowercase high", input: "high", want: ReasoningLevelHigh},
		{name: "empty", input: "", wantErr: true},
		{name: "unknown", input: "banana", wantErr: true},
		{name: "provider value not abstract", input: "xhigh", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeReasoningLevel(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeReasoningLevel(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
