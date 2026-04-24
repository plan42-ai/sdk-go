package p42

import "testing"

func TestTaskSchedulable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		task *Task
		want bool
	}{
		{
			name: "nil task",
			task: nil,
			want: false,
		},
		{
			name: "non-workstream task",
			task: &Task{State: TaskStatePending, AssignedToAI: true},
			want: false,
		},
		{
			name: "pending assigned to ai with no turns",
			task: newWorkstreamTask(),
			want: true,
		},
		{
			name: "has turns",
			task: func() *Task {
				t := newWorkstreamTask()
				t.LastTurnIndex = intPtr(0)
				return t
			}(),
			want: false,
		},
		{
			name: "not assigned to ai",
			task: func() *Task {
				t := newWorkstreamTask()
				t.AssignedToAI = false
				return t
			}(),
			want: false,
		},
		{
			name: "deleted task",
			task: func() *Task {
				t := newWorkstreamTask()
				t.Deleted = true
				return t
			}(),
			want: false,
		},
		{
			name: "not pending",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateExecuting
				return t
			}(),
			want: false,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.task.Schedulable(); got != tt.want {
				t.Fatalf("Schedulable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskBlocking(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		task *Task
		want bool
	}{
		{
			name: "nil task",
			task: nil,
			want: false,
		},
		{
			name: "non-workstream task",
			task: &Task{State: TaskStatePending},
			want: false,
		},
		{
			name: "deleted task",
			task: func() *Task {
				t := newWorkstreamTask()
				t.Deleted = true
				return t
			}(),
			want: false,
		},
		{
			name: "human pending",
			task: func() *Task {
				t := newWorkstreamTask()
				t.AssignedToAI = false
				return t
			}(),
			want: true,
		},
		{
			name: "human completed",
			task: func() *Task {
				t := newWorkstreamTask()
				t.AssignedToAI = false
				t.State = TaskStateCompleted
				return t
			}(),
			want: false,
		},
		{
			name: "ai pending",
			task: newWorkstreamTask(),
			want: true,
		},
		{
			name: "ai executing",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateExecuting
				return t
			}(),
			want: true,
		},
		{
			name: "ai awaiting code review without prs",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateAwaitingCodeReview
				t.RepoInfo = map[string]*RepoInfo{"repo": nil}
				return t
			}(),
			want: true,
		},
		{
			name: "ai awaiting code review with prs",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateAwaitingCodeReview
				pr := "https://example.com/pr/1"
				t.RepoInfo = map[string]*RepoInfo{"repo": {PRLink: &pr}}
				return t
			}(),
			want: false,
		},
		{
			name: "ai completed",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateCompleted
				return t
			}(),
			want: false,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.task.Blocking(); got != tt.want {
				t.Fatalf("Blocking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskAnchor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		task *Task
		want bool
	}{
		{
			name: "nil task",
			task: nil,
			want: false,
		},
		{
			name: "non-workstream task",
			task: &Task{State: TaskStateAwaitingCodeReview},
			want: false,
		},
		{
			name: "deleted task",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateAwaitingCodeReview
				t.Deleted = true
				return t
			}(),
			want: false,
		},
		{
			name: "wrong state",
			task: newWorkstreamTask(),
			want: false,
		},
		{
			name: "awaiting code review without prs",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateAwaitingCodeReview
				return t
			}(),
			want: false,
		},
		{
			name: "awaiting code review with prs",
			task: func() *Task {
				t := newWorkstreamTask()
				t.State = TaskStateAwaitingCodeReview
				pr := "https://example.com/pr/2"
				t.RepoInfo = map[string]*RepoInfo{"repo": {PRLink: &pr}}
				return t
			}(),
			want: true,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.task.Anchor(); got != tt.want {
				t.Fatalf("Anchor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newWorkstreamTask() *Task {
	return &Task{
		WorkstreamID: strPtr("ws"),
		State:        TaskStatePending,
		AssignedToAI: true,
	}
}

func strPtr(v string) *string {
	return &v
}

func intPtr(v int) *int {
	return &v
}
