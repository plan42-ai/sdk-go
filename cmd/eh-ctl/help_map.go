package main

var helpMap = map[string]string{
	"github add-connection": `
--- Input JSON Schema ---

{
    "Private": bool,
    "RunnerID" : "*string",
    "GithubUserLogin" : "*string",
    "GithubUserID" : *int
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
    "State"  : "*string",
    "StateExpiry" : "*string"
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
    "RunnerID" : "string",
    "GithubConnectionID" : "string"
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
	"task create": `

--- Input JSON Schema ---
{
    "Title": "string",
    "EnvironmentID": "*string",
    "Prompt": "*string",
    "Parallel": "bool",
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI": "bool",
    "RepoInfo": *{
        "org/repo": {
            "PRLink": "*string",
            "PRID": "*string",
            "PRNumber": *int,
            "FeatureBranch": "string",
            "TargetBranch": "string"
        }
    },
    "State": "*TaskState"
}

--- ModelType Enum Values ---

* Codex Mini
* O3
* O3 Pro
* Claude 4 Opus
* Claude 4 Sonnet

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
--- Input JSON Schema ---

{
    "Title": "*string",
    "EnvironmentID": "*string",
    "Prompt": "*string",
    "Parallel": "*bool",
    "Model": "*ModelType",
    "AssignedToTenantID": "*string",
    "AssignedToAI": "*bool",
    "RepoInfo": *{
        "org/repo": {
            "PRLink": "*string",
            "PRID": "*string",
            "PRNumber": *int,
            "FeatureBranch": "string",
            "TargetBranch": "string"
         }
    },
    "State": "*TaskState",
    "BeforeTaskID": "*string",
    "AfterTaskID": "*string",
    "Deleted": "*bool"
}

--- ModelType Enum Values ---

* Codex Mini
* O3
* O3 Pro
* Claude 4 Opus
* Claude 4 Sonnet

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
    "Prompt"": "string"
}
`,
	"logs upload": `
--- Input JSON Schema ---

{
    "Index": int,
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
