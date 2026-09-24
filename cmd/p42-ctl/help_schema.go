package main

import (
	"reflect"
	"strings"
	"time"

	"github.com/plan42-ai/sdk-go/p42"
)

// schemaIndent is the per-level indentation used when rendering JSON schema bodies.
const schemaIndent = "    "

// repoMapKey is the placeholder key used when rendering string-keyed maps in
// request bodies. Every map in the documented request types is keyed by a
// repository in "org/repo" form (for example RepoInfo and CommitInfo).
const repoMapKey = "org/repo"

// inputSchemaHeader is the default section header for a request body schema.
const inputSchemaHeader = "Input JSON Schema"

// timeType is used to detect time.Time fields, which marshal to JSON strings.
var timeType = reflect.TypeOf(time.Time{})

// enumValues maps an enum type to its allowed values in documentation order.
// The values reference the p42 constants so that a rename or value change fails
// to compile, keeping the generated help in sync with the SDK. Adding a brand
// new enum constant still requires appending it to the relevant slice here.
var enumValues = map[reflect.Type][]string{
	reflect.TypeOf(p42.ModelType("")): {
		string(p42.ModelTypeGpt51Codex),
		string(p42.ModelTypeGpt51CodexMax),
		string(p42.ModelTypeGpt52Codex),
		string(p42.ModelTypeGpt53Codex),
		string(p42.ModelTypeGpt54),
		string(p42.ModelTypeGpt54OneM),
		string(p42.ModelTypeChatGpt55),
		string(p42.ModelTypeChatGpt55OneM),
		string(p42.ModelTypeClaude45Opus),
		string(p42.ModelTypeClaude46Opus),
		string(p42.ModelTypeClaude47Opus),
		string(p42.ModelTypeClaude48Opus),
	},
	reflect.TypeOf(p42.TaskState("")): {
		string(p42.TaskStatePending),
		string(p42.TaskStateExecuting),
		string(p42.TaskStateAwaitingCodeReview),
		string(p42.TaskStateCompleted),
	},
	reflect.TypeOf(p42.ReasoningLevel("")): {
		string(p42.ReasoningLevelLow),
		string(p42.ReasoningLevelMedium),
		string(p42.ReasoningLevelHigh),
		string(p42.ReasoningLevelMax),
	},
}

// schemaSection describes a single "--- <header> ---" block within a command's
// help text.
type schemaSection struct {
	// header is the text rendered between the dashes, for example
	// "Input JSON Schema" or "Input JSON Schema (with --workstream-id)".
	header string
	// typ is the request struct type documented by the section. It is ignored
	// when raw is set.
	typ reflect.Type
	// note is optional explanatory prose rendered before the JSON body.
	note string
	// raw, when non-empty, is used verbatim as the body instead of reflecting typ.
	raw string
}

// commandSchema describes all schema sections for a single CLI command.
type commandSchema struct {
	sections []schemaSection
}

// enumCollector records the enum types encountered while rendering, preserving
// first-seen order so that enum sections are emitted deterministically.
type enumCollector struct {
	seen  map[reflect.Type]bool
	order []reflect.Type
}

func newEnumCollector() *enumCollector {
	return &enumCollector{seen: map[reflect.Type]bool{}}
}

func (e *enumCollector) add(t reflect.Type) {
	if e.seen[t] {
		return
	}
	e.seen[t] = true
	e.order = append(e.order, t)
}

// isEnum reports whether t is a registered enum type.
func isEnum(t reflect.Type) bool {
	_, ok := enumValues[t]
	return ok
}

// jsonName returns the JSON object key for a struct field, honoring the json tag.
func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "" {
		return f.Name
	}
	return name
}

// visibleFields returns the JSON-visible fields of a struct, flattening embedded
// (anonymous) structs the way encoding/json does. Unexported fields and fields
// tagged json:"-" are skipped, which naturally drops embedded auth and feature
// flag helpers whose fields are all excluded from JSON.
func visibleFields(t reflect.Type) []reflect.StructField {
	var out []reflect.StructField
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
				out = append(out, visibleFields(ft)...)
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// fieldStar returns a leading "*" when a pointer field should be annotated as
// nullable. Only scalar and list fields are annotated; object and map fields are
// always rendered by shape.
func fieldStar(ptr, fieldLevel bool) string {
	if ptr && fieldLevel {
		return "*"
	}
	return ""
}

// renderValue renders the schema representation of a value of type t. When
// fieldLevel is true the value sits directly after a `"name": ` prefix and
// pointer scalars/lists are annotated with a leading "*". pad is the indentation
// of the enclosing container, used to align multi-line output.
func renderValue(t reflect.Type, pad string, enums *enumCollector, fieldLevel bool) string {
	ptr := false
	if t.Kind() == reflect.Pointer {
		ptr = true
		t = t.Elem()
	}
	star := fieldStar(ptr, fieldLevel)

	switch {
	case t == timeType:
		return `"` + star + `string"`
	case t.Kind() == reflect.String:
		if isEnum(t) {
			enums.add(t)
			return `"` + star + t.Name() + `"`
		}
		return `"` + star + `string"`
	case t.Kind() == reflect.Bool:
		return star + "bool"
	case isIntKind(t.Kind()):
		return star + "int"
	case isFloatKind(t.Kind()):
		return star + "float"
	case t.Kind() == reflect.Slice:
		return star + renderSlice(t, pad, enums)
	case t.Kind() == reflect.Map:
		return renderMap(t, pad, enums)
	case t.Kind() == reflect.Struct:
		return renderObject(t, pad, enums)
	default:
		return `"` + t.Kind().String() + `"`
	}
}

func isIntKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func isFloatKind(k reflect.Kind) bool {
	return k == reflect.Float32 || k == reflect.Float64
}

// renderSlice renders a slice type. Struct and map elements are expanded across
// multiple lines; scalar elements render inline.
func renderSlice(t reflect.Type, pad string, enums *enumCollector) string {
	elem := t.Elem()
	deref := elem
	if deref.Kind() == reflect.Pointer {
		deref = deref.Elem()
	}
	multiline := (deref.Kind() == reflect.Struct && deref != timeType) || deref.Kind() == reflect.Map
	if multiline {
		inner := pad + schemaIndent
		return "[\n" + inner + renderValue(elem, inner, enums, false) + "\n" + pad + "]"
	}
	return "[" + renderValue(elem, pad, enums, false) + "]"
}

// renderMap renders a string-keyed map using a representative placeholder key.
func renderMap(t reflect.Type, pad string, enums *enumCollector) string {
	inner := pad + schemaIndent
	value := renderValue(t.Elem(), inner, enums, false)
	return "{\n" + inner + `"` + repoMapKey + `": ` + value + "\n" + pad + "}"
}

// renderObject renders a struct as a JSON object body. pad is the indentation of
// the closing brace; fields are indented one level deeper.
func renderObject(t reflect.Type, pad string, enums *enumCollector) string {
	fields := visibleFields(t)
	if len(fields) == 0 {
		return "{}"
	}
	fieldPad := pad + schemaIndent
	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		value := renderValue(f.Type, fieldPad, enums, true)
		lines = append(lines, fieldPad+`"`+jsonName(f)+`": `+value)
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n" + pad + "}"
}

// render renders a single section, recording any enum types it references.
func (s schemaSection) render(enums *enumCollector) string {
	var b strings.Builder
	b.WriteString("--- " + s.header + " ---\n\n")
	if s.note != "" {
		b.WriteString(s.note + "\n\n")
	}
	if s.raw != "" {
		b.WriteString(s.raw)
	} else {
		b.WriteString(renderObject(s.typ, "", enums))
	}
	b.WriteString("\n")
	return b.String()
}

// renderEnumSection renders the "--- <Type> Enum Values ---" block for t.
func renderEnumSection(t reflect.Type) string {
	var b strings.Builder
	b.WriteString("--- " + t.Name() + " Enum Values ---\n\n")
	for _, v := range enumValues[t] {
		b.WriteString("* " + v + "\n")
	}
	return b.String()
}

// render renders the full help text for a command: every schema section followed
// by the enum sections referenced by any of them.
func (c commandSchema) render() string {
	enums := newEnumCollector()
	sections := make([]string, 0, len(c.sections))
	for _, s := range c.sections {
		sections = append(sections, s.render(enums))
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(strings.Join(sections, "\n"))
	for _, et := range enums.order {
		b.WriteString("\n")
		b.WriteString(renderEnumSection(et))
	}
	return b.String()
}

// buildHelpMap generates the command help text lookup from schemaSpecs. It is
// evaluated once at startup so that the help output can never drift from the
// request structs it documents.
func buildHelpMap() map[string]string {
	out := make(map[string]string, len(schemaSpecs))
	for cmd, spec := range schemaSpecs {
		out[cmd] = spec.render()
	}
	return out
}
