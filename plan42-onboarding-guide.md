# Plan42.ai — Getting Started Guide

This guide walks you through setting up your Plan42 account, configuring your first environment, and using the platform to run ad-hoc tasks and orchestrated workstreams.

---

## 1. Creating Your Account

### GitHub Cloud Users

1. Navigate to [dev.plan42.ai](https://dev.plan42.ai).
2. You'll see the sign-in page. Click **Login with Google** to authenticate.
3. After signing in, you'll be redirected to the **Configure GitHub** page. Click **Connect to GitHub** to link your GitHub account via OAuth.
4. GitHub will ask you to authorize the Plan42 GitHub App and select which organizations and repositories to grant access to. Complete the authorization flow.
5. Once connected, the page will show your linked GitHub username (e.g., "Connected as **@yourname**"). You can manage app permissions via **Open GitHub Settings** at any time.
6. You're now ready to use Plan42. The platform will redirect you to the main task page.

### GitHub Enterprise Server (GHES) Users

Connecting to an internal GitHub Enterprise Server instance requires a **remote runner** and some manual onboarding. Contact the Plan42 team to set up a private GitHub connection. Once configured, remote runners running in your infrastructure will bridge Plan42's cloud platform with your private GHES instance — your credentials and source code never leave your network.

---

## 2. Setting Up Environments

Before you can run any tasks, you need at least one **environment**. Environments define which repositories are available, what secrets and configuration the agent has access to, and what network endpoints the agent is allowed to reach.

### Creating an Environment

1. Click **Environments** in the top navigation bar.
2. Click **Create environment**.
3. Fill in the environment settings:

| Setting | Description |
|---|---|
| **Repos** | Select one or more GitHub repositories. Use the org selector to browse your connected organizations and pick repos. These are the repos the AI agent will check out and work against. |
| **Name** | A human-readable name for the environment (auto-populated from the first selected repo, but editable). |
| **Context** | Optional free-text instructions for the AI about this environment (up to 25,000 characters). Use this to provide architectural context, coding conventions, or anything the agent should know. |
| **Image** | The agent container image. Defaults to `plan42/agent`. Leave as-is unless instructed otherwise. |
| **Environment variables** | Key-value pairs that are available to the agent and to bootstrap scripts during execution. |
| **Secrets** | Key-value pairs that are available **only to bootstrap scripts** — the AI model never sees secret values. Use these for API keys, tokens, or credentials needed during setup. |
| **Bootstrap script** | A shell script (up to 5,000 characters) that runs before the agent starts. Use it to install dependencies, configure tools, or set up the workspace. |
| **Allowed endpoints** | Network endpoints the agent is allowed to reach. On cloud runners, all outbound traffic is filtered through Plan42's proxy — only explicitly allowed endpoints are reachable. Use this to grant access to package registries (npm, PyPI, etc.), internal APIs, or other services. |

4. Click **Create Environment**.

### Editing an Environment

From the **Environments** page, click any environment row to view its details, then click **Edit** to modify settings. You can add or remove repos, update secrets, change the bootstrap script, or adjust allowed endpoints at any time.

### Multiple Environments

You can create as many environments as you need. Common patterns include:

- **One environment per repo** — simple and focused.
- **One environment per project** — when a project spans multiple repos.
- **Separate environments for different security postures** — e.g., a frontend environment with npm access vs. a backend environment with no external network access.

---

## 3. The "What do you want to do today?" Page (Ad-Hoc Tasks)

After onboarding, the main page greets you with **"What do you want to do today?"** — a large prompt area where you can describe what you want the AI to do. This is the hub for both ad-hoc tasks and workstream planning.

### Running an Ad-Hoc Task

Ad-hoc tasks work similarly to other AI coding tools (like Claude Code or Codex). You describe what you want, and the AI executes it in a single session:

1. Type your prompt in the text area. Be as specific as you like — the AI works best with clear, concrete instructions.
2. Select your **Environment** from the dropdown. This determines which repos are available.
3. Select the **Branch** to work against (defaults to the repo's default branch). For multi-repo environments, click **Configure Branches...** to set per-repo branches.
4. Choose a **Model** (e.g., GPT 5.4, Claude 4.6 Opus) and a **Reasoning** level (Low, Medium, High, Max).
5. Optionally attach files (images, PDFs, documents) using the **Upload File** button — up to 25 files, 30 MB each.
6. Click **Submit**.

The task will appear in the **Tasks** list below the prompt. Click on it to watch execution in real time — you'll see the AI's log stream, tool calls, and progress. When the task completes, it opens a GitHub PR with the changes.

### Task List

Below the prompt area, you'll see your recent tasks. Each task shows its title, status, and creation time. You can:

- Click a task to view its details and execution logs.
- Toggle between **Tasks** and **Archived** to view current or archived tasks.
- Archive completed tasks you no longer need.

### Planning Workstreams from the Task Page

The same prompt interface is also where you ask Plan42 to **plan a workstream**. Instead of describing a single coding task, describe a larger objective — a feature, a refactor, a multi-step project. The AI will create a workstream with an ordered set of tasks, dependencies, and parallel groups. More on this in the next section.

---

## 4. Workstreams

Workstreams are Plan42's core orchestration feature. A workstream is an ordered sequence of tasks — like a sprint board — that Plan42 can execute automatically, parallelizing work where possible.

### Navigating to Workstreams

Click **Workstreams** in the top navigation bar to open the workstreams page.

<!-- See specs/003-workstreams/workstreams-page.png for the full workstreams page layout -->

### Creating a Workstream

There are two ways to create a workstream:

1. **Manually** — Click **+ Add New Workstream** at the top of the workstreams page. This creates an empty workstream in paused mode.
2. **Using AI** — Click **Plan a Workstream using AI** (or use the planning icon on an existing workstream) to open the task page, where you can describe a sprint-level objective and let the AI plan the workstream for you.

### Workstream States (Color Coding)

<!-- See specs/003-workstreams/pause.png for pause/play controls -->

Workstreams are color-coded:

- **Green** — Running. Tasks are automatically executed as their predecessors complete.
- **Yellow** — Paused. No automatic execution. New workstreams start in this state so you can review the plan before execution begins.
- **Red** — Archived. The workstream is archived and hidden by default.

### Workstream Layout

Each workstream card has three sections:

1. **Header** — Shows the workstream name (click to edit), a collapse/expand chevron, and action buttons.
2. **Description** — A markdown description of the workstream's purpose (click to edit). Shows "Click to edit description" if empty.
3. **Swim Lanes** — Four columns representing task states: **Pending**, **Executing**, **Review**, and **Completed**. Tasks flow left to right as they progress.

<!-- See specs/003-workstreams/worksteam-list.png for the workstream list view -->

### Action Buttons

<!-- See specs/003-workstreams/action-buttons.png -->

Each workstream header has four action buttons:

| Button | Action |
|---|---|
| **Plan** (pencil icon) | Opens a planning session — use AI to add or modify tasks. |
| **Play/Pause** | Toggles automatic execution. Click **Play** to start executing pending tasks; click **Pause** to stop. |
| **Archive/Unarchive** | Archives (soft-deletes) or restores a workstream. |
| **Details** (three dots) | Opens the workstream detail page with full settings. |

### Task Cards

<!-- See specs/003-workstreams/task-card.png -->

Each task in a swim lane is displayed as a card with:

- **Title** — The task name. Click the edit icon (pencil) to rename inline.
- **Assignee** — Shows who the task is assigned to: **Plan42** (robot icon), **You** (your profile picture), or **Unassigned** (?). Click to reassign.
- **Parallel toggle** — A switch to mark the task as parallelizable. When enabled, adjacent parallel tasks execute together as a group.
- **Short name link** — (e.g., "ORG-3") Click to open the full task detail page.
- **Archive button** — Archive the task.

<!-- See specs/003-workstreams/reassign.png for the assignee dropdown -->

### Reordering Tasks (Drag and Drop)

Tasks can be reordered within a swim lane by dragging them up or down. This changes their execution order — earlier tasks execute first.

<!-- See specs/003-workstreams/drag-vertical.png for vertical reordering -->

Tasks can also be moved between swim lanes by dragging them horizontally. This changes their state (e.g., moving a task from Pending to Completed).

<!-- See specs/003-workstreams/drag-siwm-lane.png for cross-lane dragging -->

### Adding Tasks

Click the **+ New Task** button at the bottom of the Pending swim lane to add a new task to the workstream.

### Parallel Execution

By default, tasks execute serially in the order they're listed. To run tasks in parallel:

1. Make sure the tasks you want to parallelize are **adjacent** in the list.
2. Toggle the **Parallel** switch on each task in the group.
3. Adjacent parallel tasks will execute together as a single group when the scheduler reaches them.

**Example execution order:**

```
Task A              → executes first
Task B (parallel)   ┐
Task C (parallel)   ┘ execute together
Task D              → executes after B and C complete
```

### Starting Execution

New workstreams start **paused**. This gives you time to review the plan, adjust task order, set environments, and configure assignments. When you're ready:

1. Make sure AI-assigned tasks have an environment set.
2. Click the **Play** button on the workstream header.
3. Plan42 will begin executing tasks automatically — scheduling them in order, parallelizing where marked, and creating GitHub PRs.

---

## 5. Workstream Task Detail Page

Click any task's short name link (e.g., "ORG-3") to open the full task detail page. This page provides complete control over a single task:

### Task Properties

- **Title** — Click to edit the task title inline.
- **Status** — A dropdown showing the current state: Pending, Executing, Awaiting Code Review, or Completed. You can manually change the state.
- **Assignee** — Assign to Plan42 (AI), yourself, or leave unassigned. Only AI-assigned tasks are automatically executed.
- **Environment and Branches** — For AI-assigned tasks, select which environment and target branches to use.
- **Spec** — The task specification (prompt). This is the instruction the AI receives when executing the task. Click to edit — write in markdown for clarity.
- **Parallel** — Toggle whether this task can run in parallel with adjacent parallel tasks.

### Activity Section

The bottom of the task page shows the **Activity** section — a history of all turns (execution rounds) for the task.

Each turn shows:
- A link to view full execution logs (real-time streaming view).
- The turn's status (Running, Done, Failed).
- A summary of what happened during execution.

### Requesting Changes

There are three ways to request changes or trigger additional work on a task:

1. **From the task page** — Type a follow-up prompt in the Activity section and submit. This creates a new turn (execution round) for the task.
2. **From the GitHub PR** — Leave a comment on the PR with `/plan42` followed by your instructions. The Plan42 webhook picks this up and creates a new turn automatically.
3. **From GitHub PR review comments** — Leave inline review comments on specific code lines. Use `/plan42` in the PR thread to trigger a new turn that addresses your feedback.

---

## 6. The Review and Merge Loop

Once a task completes, its PR is ready for review:

1. The task moves to the **Review** swim lane on the workstreams page.
2. Open the GitHub PR to review the code. Plan42 has already run multiple rounds of automated code review before the PR reaches you.
3. If changes are needed, comment on the PR (using `/plan42` to trigger a new round) or use the task page to submit follow-up instructions.
4. When satisfied, **approve and merge** the PR on GitHub.
5. The task automatically moves to **Completed**.
6. Plan42 detects the merge, rebases any downstream stacked PRs, and **schedules the next tasks** in the workstream automatically.

This loop continues until the workstream is complete. You can have **multiple workstreams running simultaneously** across different repositories — Plan42 handles all the orchestration.

---

## 7. Filtering and Managing Workstreams

### Filter Panel

Click the **search icon** on the left sidebar of the workstreams page to open the filter panel. From here you can:

- **Show All** — Toggle to display all workstreams or only selected ones.
- **Show Archived** — Include archived (red) workstreams in the view.
- **Select/Deselect** individual workstreams to show or hide.
- **Filter tasks** by search text, assignee (AI, You, Unassigned), or archived status.

### Workstream Settings

From the workstream detail page (click the three-dot icon), you can access additional settings:

- **Short names** — Each workstream gets a short name (e.g., "ORG") used in task references like `ORG-1`, `ORG-2`. These are auto-generated but can be customized.
- **Workstream description** — A detailed description of the workstream's goals and context.

---

## 8. Key Concepts Summary

| Concept | Description |
|---|---|
| **Environment** | Configuration defining repos, secrets, network access, and bootstrap scripts for AI execution. |
| **Ad-hoc Task** | A single AI coding task — describe what you want, AI executes it and opens a PR. |
| **Workstream** | An ordered sequence of tasks (like a sprint) that Plan42 executes automatically. |
| **Task** | A single unit of work within a workstream, assigned to AI or a human. |
| **Turn** | One execution round of a task. Tasks can have multiple turns (initial execution + follow-ups). |
| **Parallel group** | Adjacent tasks marked as parallel that execute simultaneously. |
| **Paused/Running** | Workstreams start paused. Click Play to begin automatic execution. |
| **Swim lanes** | Four columns (Pending, Executing, Review, Completed) showing task progress. |
| **Short name** | A human-readable prefix (e.g., "ORG") used in task references like `ORG-1`. |
| **Remote runner** | An agent running in your own infrastructure for GHES access or credential isolation. |
