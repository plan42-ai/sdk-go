package main

// #nosec:  G101: Potential hardcoded credentials
//
//	No hard coded creds here. Just json fields with names like "token", "oauth", etc, that match
//	the linter's regex.
var helpMap = map[string]string{
	"github add-connection": `
--- Input JSON Schema ---

{
    "Private": bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "Name": "*string"
}
`,
	"github update-connection": `
--- Input JSON Schema ---

{
    "Private": *bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int,
    "OAuthToken": "*string",
    "RefreshToken": "*string",
    "TokenExpiry": "*string",
    "State"  : "*string",
    "StateExpiry" : "*string",
    "Name" : "*string"
}
`,
	"github update-org": `
--- Input JSON Schema ---

{
    "OrgName": "*string",
    "InstallationID": "*int",
    "Deleted":  "*bool"  
}
`,
	"github update-tenant-creds": `
--- Input JSON Schema ---

{
	"SkipOnboarding" : *bool,
	"OAuthToken" : "*string",
	"RefreshToken": "*string",
	"TokenExpiry" : "*string",
	"State " : "*string",
	"StateExpiry": "*string",
	"GithubUserLogin": "*string",
	"GithubUserID": *int
`,
	"environment create": `
--- Input JSON Schema ---

{
    "Name": "string",
    "Description": "string",
    "Context": "string",
    "Repos": ["org/repo", ...],
    "SetupScript": "string",
    "DockerImage": "*string",
    "AllowedHosts": [string],
    "EnvVars": [
        {
            "Name": "string",
            "Value": "string",
            "IsSecret": bool
        }
    ],
    "RunnerID" : "*string",
    "GithubConnectionID" : "*string"
}`,
	"environment update": `
--- Input JSON Schema ---

{
    "Name": "*string",
    "Description": "*string",
    "Context": "*string",
    "Repos": *["org/repo",...],
    "SetupScript": "string",
    "DockerImage": "string",
    "AllowedHosts": *[string],
    "EnvVars": *[
        {
            "Name": "string",
            "Value": "string",
            "IsSecret": bool
        }
    ],
    "Deleted": *bool,
    "RunnerID" : "*string",
    "GithubConnectionID" : "*string"
}
`,
	"tenant update": `
--- Input JSON Schema ---

{
    "DefaultRunnerID": "*string",
    "DefaultGithubConnectionID": "*string"
}
`,
	"task create": `

--- Input JSON Schema (without --workstream-id) ---
{
    "Title": "string",
    "EnvironmentID": "string",
    "Prompt": "string",
    "Model": "*ModelType",
    "RepoInfo": {
        "org/repo": {
            "PRLink": "*string",
            "PRID": "*string",
            "PRNumber": *int,
            "PRStatus": "*string",
            "PRStatusUpdatedAt": "*string",
            "FeatureBranch": "string",
            "TargetBranch": "string"
        }
    },
    "FileIDs": []
}

--- Input JSON Schema (with --workstream-id) ---
{
    "Title": "string",
    "EnvironmentID": "*string",
    "Prompt": "*string",
    "Parallel": bool,
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI": bool,
    "RepoInfo": {
        "org/repo": {
            "PRLink": "*string",
            "PRID": "*string",
            "PRNumber": *int,
            "PRStatus": "*string",
            "PRStatusUpdatedAt": "*string",
            "FeatureBranch": "string",
            "TargetBranch": "string"
        }
    },
    "State": "*TaskState",
    "FileIDs": []
}

--- ModelType Enum Values ---

* GPT-5.1 Codex
* GPT-5.1 Codex Max
* GPT-5.2 Codex
* GPT-5.3 Codex
* GPT 5.4
* GPT 5.4 (1M)
* Chat GPT 5.5
* Chat GPT 5.5 (1M)
* Claude 4.5 Opus
* Claude 4.6 Opus
* Claude 4.7 Opus
* Claude 4.8 Opus

--- TaskState Enum Values ---

* Pending
* Executing
* Awaiting Code Review
* Completed
* Failed

`,
	"task search": `

--- Input JSON Schema ---

{}

`,
	"task update": `
--- Input JSON Schema (without --workstream-id) ---

{
    "Title": "*string",
    "Prompt": "*string",
    "Model": "*ModelType",
    "RepoInfo": {
        "org/repo": {
            "PRLink": "*string",
            "PRID": "*string",
            "PRNumber": *int,
            "PRStatus": "*string",
            "PRStatusUpdatedAt": "*string",
            "FeatureBranch": "string",
            "TargetBranch": "string"
         }
    },
    "Deleted": "*bool",
    "NewFileIDs": *[]string
}

--- Input JSON Schema (with --workstream-id) ---

{
    "Title": "*string",
    "EnvironmentID": "*string",
    "Prompt": "*string",
    "Parallel": "*bool",
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI": "*bool",
    "RepoInfo": {
        "org/repo": {
            "PRLink": "*string",
            "PRID": "*string",
            "PRNumber": *int,
            "PRStatus": "*string",
            "PRStatusUpdatedAt": "*string",
            "FeatureBranch": "string",
            "TargetBranch": "string"
         }
    },
    "State": "*TaskState",
    "BeforeTaskID": "*string",
    "AfterTaskID": "*string",
    "Deleted": "*bool",
    "NewFileIDs": []string,
    "RemoveFileIDs": []string
}

--- ModelType Enum Values ---

* GPT-5.1 Codex
* GPT-5.1 Codex Max
* GPT-5.2 Codex
* GPT-5.3 Codex
* GPT 5.4
* GPT 5.4 (1M)
* Chat GPT 5.5
* Chat GPT 5.5 (1M)
* Claude 4.5 Opus
* Claude 4.6 Opus
* Claude 4.7 Opus
* Claude 4.8 Opus

--- TaskState Enum Values ---

* Pending
* Executing
* Awaiting Code Review
* Completed
* Failed

`,
	"turn create": `
--- Input JSON Schema ---

{
    "Prompt": "string",
    "WorkstreamID": "*string",
    "AdditionalFileIDs": []
}
`,
	"turn update": `
--- Input JSON Schema ---

{
    "PreviousResponseID": "*string",
    "CommitInfo": {
        "org/repo": {
            "BaselineCommitHash": "*string",
            "LastCommitHash": "*string"
        }
    },
    "WorkstreamID": "*string",
    "Status": "*string",
    "OutputMessage": "*string",
    "ErrorMessage": "*string"
}
`,
	"logs upload": `

--- Input JSON Schema ---

{
    "Index": int,
    "WorkstreamID": "*string",
    "Logs": [
        {
            "Timestamp": "string",
            "Message": "string"
        }
    ]
}
`,
	"feature-flag add": `

--- Input JSON Schema ---

{
    "Description": "string",
    "DefaultPct" " float
}
`,
	"feature-flag update": `

--- Input JSON Schema ---

{
    "Description": "*string",
    "DefaultPct": "*float",
    "Deleted": "*bool"
}`,
	"workstream create": `
--- Input JSON Schema ---

{
    "Name": "string",
    "Description": "string"
    "DefaultShortName": "string",
}
`,
	"workstream update": `

--- Input JSON Schema ---

{
    "Name": "*string",
    "Description": "*string",
    "Paused": "*bool",
    "Deleted": "*bool",
    "DefaultShortName": "*string"
}`,
	"runner create": `

--- Input JSON Schema ---

{
    "Name": "string",
    "Description": "*string",
    "IsCloud" : bool,
    "RunsTasks" : bool,
    "ProxiesGithub":bool    
}
`,
	"runner update": `
--- Input JSON Schema ---

{
    "Name": "*string",
    "Description": "*string",
    "IsCloud" : "*bool",
    "RunsTasks" : "*bool",
    "ProxiesGithub":*bool,
    "Deleted": "*bool"    
}
`,
}
