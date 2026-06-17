package p42

import "testing"

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
