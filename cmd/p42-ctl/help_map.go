package main

import (
	"reflect"

	"github.com/plan42-ai/sdk-go/p42"
)

// schemaSpecs maps each CLI command that accepts a --json body to the request
// struct(s) it decodes. The help text shown by `p42-ctl <command> --help` is
// generated from these specs by reflecting over the referenced structs, so the
// documented field names, types, and enum values stay in sync with the SDK.
//
// Only commands that read JSON appear here. Commands that build their request
// solely from CLI flags (for example `feature-flag add` and `github add-org`)
// intentionally have no entry.
var schemaSpecs = map[string]commandSchema{
	"github add-connection": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.CreateGithubConnectionRequest{})},
	}},
	"github update-connection": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateGithubConnectionRequest{})},
	}},
	"github update-org": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateGithubOrgRequest{})},
	}},
	"environment create": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.CreateEnvironmentRequest{})},
	}},
	"environment update": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateEnvironmentRequest{})},
	}},
	"tenant update": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateTenantRequest{})},
	}},
	"task create": {sections: []schemaSection{
		{header: "Input JSON Schema (without --workstream-id)", typ: reflect.TypeOf(p42.CreateTaskRequest{})},
		{header: "Input JSON Schema (with --workstream-id)", typ: reflect.TypeOf(p42.CreateWorkstreamTaskRequest{})},
	}},
	"task update": {sections: []schemaSection{
		{header: "Input JSON Schema (without --workstream-id)", typ: reflect.TypeOf(p42.UpdateTaskRequest{})},
		{header: "Input JSON Schema (with --workstream-id)", typ: reflect.TypeOf(p42.UpdateWorkstreamTaskRequest{})},
	}},
	"task search": {sections: []schemaSection{
		{
			header: inputSchemaHeader,
			note:   "Optional. An arbitrary JSON object sent as the search request body.",
			raw:    "{}",
		},
	}},
	"turn create": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.CreateTurnRequest{})},
	}},
	"turn update": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateTurnRequest{})},
	}},
	"logs upload": {sections: []schemaSection{
		{
			header: inputSchemaHeader,
			note:   "Reads a stream of JSON log entries (one JSON object per entry), each of the form:",
			typ:    reflect.TypeOf(p42.TurnLog{}),
		},
	}},
	"feature-flag update": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateFeatureFlagRequest{})},
	}},
	"workstream create": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.CreateWorkstreamRequest{})},
	}},
	"workstream update": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateWorkstreamRequest{})},
	}},
	"runner create": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.CreateRunnerRequest{})},
	}},
	"runner update": {sections: []schemaSection{
		{header: inputSchemaHeader, typ: reflect.TypeOf(p42.UpdateRunnerRequest{})},
	}},
}

// helpMap holds the generated help text keyed by CLI command. printHelp consults
// it after Kong's default help output.
var helpMap = buildHelpMap()
