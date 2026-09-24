package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// jsonTagNames independently derives the top-level JSON field names of t,
// flattening embedded structs and skipping json:"-" fields. It mirrors
// encoding/json semantics without reusing the help generator so the tests catch
// drift between the documented schema and the request struct.
func jsonTagNames(t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				names = append(names, jsonTagNames(ft)...)
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		names = append(names, name)
	}
	return names
}

// topLevelKeys extracts the object keys indented exactly one level (schemaIndent)
// from a rendered schema body.
func topLevelKeys(body string) []string {
	var keys []string
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, schemaIndent+`"`) {
			continue
		}
		rest := line[len(schemaIndent)+1:]
		end := strings.Index(rest, `"`)
		if end < 0 {
			continue
		}
		keys = append(keys, rest[:end])
	}
	return keys
}

// TestHelpSchemaFieldsMatchStructTags verifies that, for every command that
// documents a request struct, the top-level documented field names exactly match
// the struct's JSON tags. This fails whenever a field is added, renamed, or
// removed on the request type without updating the docs.
func TestHelpSchemaFieldsMatchStructTags(t *testing.T) {
	for cmd, spec := range schemaSpecs {
		for _, sec := range spec.sections {
			if sec.typ == nil {
				continue
			}
			body := renderObject(sec.typ, "", newEnumCollector())
			require.ElementsMatchf(
				t,
				jsonTagNames(sec.typ),
				topLevelKeys(body),
				"command %q section %q (%s)",
				cmd, sec.header, sec.typ,
			)
		}
	}
}

// TestHelpSchemaCommands locks the exact set of commands that expose a JSON
// schema so stale entries (for example the removed "feature-flag add" and
// "github update-tenant-creds") cannot creep back in.
func TestHelpSchemaCommands(t *testing.T) {
	want := []string{
		"github add-connection",
		"github update-connection",
		"github update-org",
		"environment create",
		"environment update",
		"tenant update",
		"task create",
		"task update",
		"task search",
		"turn create",
		"turn update",
		"logs upload",
		"feature-flag update",
		"workstream create",
		"workstream update",
		"runner create",
		"runner update",
	}
	got := make([]string, 0, len(helpMap))
	for cmd := range helpMap {
		got = append(got, cmd)
	}
	require.ElementsMatch(t, want, got)
}

// TestHelpSchemaEnumFieldsRegistered ensures every named string field reachable
// from a documented request struct is a registered enum, so a new enum field can
// never be added without also documenting its allowed values.
func TestHelpSchemaEnumFieldsRegistered(t *testing.T) {
	seen := map[reflect.Type]bool{}
	for cmd, spec := range schemaSpecs {
		for _, sec := range spec.sections {
			if sec.typ == nil {
				continue
			}
			for _, nt := range namedStringTypes(sec.typ, map[reflect.Type]bool{}) {
				if seen[nt] {
					continue
				}
				seen[nt] = true
				require.Truef(t, isEnum(nt), "command %q references unregistered enum type %s", cmd, nt)
			}
		}
	}
}

// namedStringTypes returns the named (non-builtin) string types reachable from t.
func namedStringTypes(t reflect.Type, visited map[reflect.Type]bool) []reflect.Type {
	if visited[t] {
		return nil
	}
	visited[t] = true
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map:
		return namedStringTypes(t.Elem(), visited)
	case reflect.String:
		if t != reflect.TypeOf("") {
			return []reflect.Type{t}
		}
		return nil
	case reflect.Struct:
		if t == timeType {
			return nil
		}
		var out []reflect.Type
		for _, f := range visibleFields(t) {
			out = append(out, namedStringTypes(f.Type, visited)...)
		}
		return out
	default:
		return nil
	}
}

// TestHelpSchemaReasoningLevelDocumented locks in the primary bug fix: reasoning
// level is documented (with its enum values) for every command whose request
// type accepts it.
func TestHelpSchemaReasoningLevelDocumented(t *testing.T) {
	for _, cmd := range []string{"task create", "task update", "turn create"} {
		help := helpMap[cmd]
		require.Containsf(t, help, `"ReasoningLevel": "*ReasoningLevel"`, "command %q", cmd)
		require.Containsf(t, help, "--- ReasoningLevel Enum Values ---", "command %q", cmd)
		for _, v := range []string{"* Low", "* Medium", "* High", "* Max"} {
			require.Containsf(t, help, v, "command %q", cmd)
		}
	}
}

// TestHelpSchemaEnvironmentIDCasing verifies the JSON tag casing fixes for the
// EnvironmentId / RunnerId / GithubConnectionId fields.
func TestHelpSchemaEnvironmentIDCasing(t *testing.T) {
	create := helpMap["task create"]
	require.Contains(t, create, `"EnvironmentId":`)
	require.NotContains(t, create, `"EnvironmentID":`)

	env := helpMap["environment create"]
	require.Contains(t, env, `"RunnerId":`)
	require.Contains(t, env, `"GithubConnectionId":`)
	require.NotContains(t, env, `"RunnerID":`)
	require.NotContains(t, env, `"GithubConnectionID":`)
}

// TestHelpSchemaTaskStateEnumValues verifies the TaskState enum lists only the
// real states and not the previously documented (nonexistent) "Failed" value.
func TestHelpSchemaTaskStateEnumValues(t *testing.T) {
	help := helpMap["task create"]
	idx := strings.Index(help, "--- TaskState Enum Values ---")
	require.GreaterOrEqual(t, idx, 0)
	section := help[idx:]
	for _, v := range []string{"* Pending", "* Executing", "* Awaiting Code Review", "* Completed"} {
		require.Contains(t, section, v)
	}
	require.NotContains(t, section, "Failed")
}

// TestHelpSchemaGoldenTenantUpdate locks the exact rendered format for a simple
// command so accidental formatting regressions are caught.
func TestHelpSchemaGoldenTenantUpdate(t *testing.T) {
	want := "\n" +
		"--- Input JSON Schema ---\n" +
		"\n" +
		"{\n" +
		"    \"DefaultRunnerID\": \"*string\",\n" +
		"    \"DefaultGithubConnectionID\": \"*string\"\n" +
		"}\n"
	require.Equal(t, want, helpMap["tenant update"])
}

// TestHelpSchemaLogsUploadUsesTurnLog verifies the logs upload schema documents a
// single TurnLog entry (the streamed shape) rather than a batch request body.
func TestHelpSchemaLogsUploadUsesTurnLog(t *testing.T) {
	help := helpMap["logs upload"]
	require.Contains(t, help, `"Timestamp": "string"`)
	require.Contains(t, help, `"Message": "string"`)
	require.Contains(t, help, `"ProviderMessage": "string"`)
	require.NotContains(t, help, `"Logs":`)
}
