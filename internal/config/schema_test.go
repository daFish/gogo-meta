package config

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseAndValidate(content string, format ConfigFormat) (*MetaConfig, error) {
	cfg, err := parseContent([]byte(content), format)
	if err != nil {
		return nil, err
	}
	if err := Validate(*cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func schemaProblems(t *testing.T, err error) []string {
	t.Helper()
	var schemaErr *SchemaError
	require.ErrorAs(t, err, &schemaErr)
	return schemaErr.Problems
}

func TestEmbeddedSchemaCompiles(t *testing.T) {
	schema, err := loadConfigSchema()
	require.NoError(t, err, "gogo.schema.json must be a valid draft 2020-12 schema")
	assert.NotNil(t, schema.validator)
}

func TestCompileConfigSchemaRejectsAnInvalidSchema(t *testing.T) {
	_, err := compileConfigSchema([]byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":5}`))
	require.Error(t, err)
}

// TestSchemaMatchesConfigStructs is the drift guard between the Go structs and
// gogo.schema.json: every json tag of MetaConfig and CommandConfig must be a
// property of the schema and vice versa, in the same order, so a field added
// on one side only fails CI. The yaml tags must match too, because the one
// schema validates both formats.
func TestSchemaMatchesConfigStructs(t *testing.T) {
	schema, err := loadConfigSchema()
	require.NoError(t, err)

	cases := []struct {
		name    string
		goType  reflect.Type
		pointer []string
	}{
		{"MetaConfig", reflect.TypeFor[MetaConfig](), nil},
		{"CommandConfig", reflect.TypeFor[CommandConfig](), []string{"properties", "commands", "additionalProperties"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := schema.root.lookup(tc.pointer)
			require.NotNil(t, node)

			var jsonNames, yamlNames []string
			for field := range tc.goType.Fields() {
				jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				yamlName, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
				jsonNames = append(jsonNames, jsonName)
				yamlNames = append(yamlNames, yamlName)
			}
			assert.Equal(t, jsonNames, node.propertyNames(), "json tags of %s vs. schema properties", tc.name)
			assert.Equal(t, jsonNames, yamlNames, "yaml tags of %s must match its json tags", tc.name)
		})
	}
}

type fencedBlock struct {
	line    int
	format  ConfigFormat
	content string
}

// configBlocks returns the ```json, ```yaml and ```yml blocks of a Markdown
// document.
func configBlocks(t *testing.T, path string) []fencedBlock {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	formats := map[string]ConfigFormat{"json": FormatJSON, "yaml": FormatYAML, "yml": FormatYAML}
	var blocks []fencedBlock
	var current *fencedBlock
	var body strings.Builder
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		switch {
		case current != nil && strings.TrimSpace(text) == "```":
			current.content = body.String()
			blocks = append(blocks, *current)
			current = nil
		case current != nil:
			body.WriteString(text + "\n")
		case strings.HasPrefix(text, "```"):
			if format, ok := formats[strings.TrimSpace(strings.TrimPrefix(text, "```"))]; ok {
				current = &fencedBlock{line: line, format: format}
				body.Reset()
			}
		}
	}
	require.NoError(t, scanner.Err())
	return blocks
}

// TestDocumentedConfigsMatchSchema validates the config examples in README.md.
// The rule: every ```json, ```yaml and ```yml block in the README is a
// complete config document and must load. A snippet that shows only part of a
// config belongs in a block of another language (```text, ```bash), so a
// broken full example can never be waved through as "partial". The block count
// is pinned so the test cannot pass vacuously when fences change.
//
// The test fixtures need no list of their own: they are inline in the tests
// and every one that is meant to load goes through parseContent, and so
// through the schema, when its test reads it.
func TestDocumentedConfigsMatchSchema(t *testing.T) {
	blocks := configBlocks(t, filepath.Join("..", "..", "README.md"))
	require.Len(t, blocks, 4, "README.md config blocks")

	for _, block := range blocks {
		t.Run(fmt.Sprintf("line %d", block.line), func(t *testing.T) {
			_, err := parseAndValidate(block.content, block.format)
			assert.NoError(t, err)
		})
	}
}

// configLayout places a config document where gogo reads it: as the primary
// config, as an overlay named with -f, or as the .gogo.local beside the
// primary. It returns the path errors must name and the result of reading.
type configLayout struct {
	name string
	read func(t *testing.T, dir string, format ConfigFormat, content string) (string, *MetaConfigResult, error)
}

var configLayouts = []configLayout{
	{"primary", func(t *testing.T, dir string, format ConfigFormat, content string) (string, *MetaConfigResult, error) {
		path := filepath.Join(dir, FilenameForFormat(format))
		writeFile(t, path, content)
		result, err := ReadMetaConfig(dir, nil)
		return path, result, err
	}},
	{"overlay", func(t *testing.T, dir string, format ConfigFormat, content string) (string, *MetaConfigResult, error) {
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{}}`)
		name := ".gogo.devops" + strings.TrimPrefix(FilenameForFormat(format), ".gogo")
		path := filepath.Join(dir, name)
		writeFile(t, path, content)
		result, err := ReadMetaConfig(dir, []string{name})
		return path, result, err
	}},
	{"local", func(t *testing.T, dir string, format ConfigFormat, content string) (string, *MetaConfigResult, error) {
		primary := FilenameForFormat(format)
		writeFile(t, filepath.Join(dir, primary), "{}")
		path := filepath.Join(dir, LocalFilenameForPrimary(primary))
		writeFile(t, path, content)
		result, err := ReadMetaConfig(dir, nil)
		return path, result, err
	}},
}

func TestSchemaRejectsConfigFiles(t *testing.T) {
	cases := []struct {
		name string
		json string
		yaml string
		want string
	}{
		{
			name: "unknown top-level key",
			json: `{"projects":{},"group":{"backend":["api"]}}`,
			yaml: "projects: {}\ngroup:\n  backend: [api]\n",
			want: `unknown key "group" (allowed: $schema, projects, ignore, groups, commands)`,
		},
		{
			name: "unknown key in the object form of a command",
			json: `{"commands":{"test":{"cmd":"npm test","parallell":true}}}`,
			yaml: "commands:\n  test:\n    cmd: npm test\n    parallell: true\n",
			want: `commands.test: unknown key "parallell" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`,
		},
		{
			name: "wrong type",
			json: `{"ignore":"node_modules"}`,
			yaml: "ignore: node_modules\n",
			want: `ignore: must be a list (got "node_modules")`,
		},
		{
			name: "out-of-range value",
			json: `{"commands":{"build":{"cmd":"make","concurrency":0}}}`,
			yaml: "commands:\n  build:\n    cmd: make\n    concurrency: 0\n",
			want: `commands.build.concurrency: must be >= 1 (got 0)`,
		},
		{
			name: "command object without cmd",
			json: `{"commands":{"build":{"parallel":true}}}`,
			yaml: "commands:\n  build:\n    parallel: true\n",
			want: `commands.build: missing required key "cmd"`,
		},
	}

	for _, format := range []ConfigFormat{FormatJSON, FormatYAML} {
		for _, layout := range configLayouts {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s/%s/%s", format, layout.name, tc.name), func(t *testing.T) {
					content := tc.json
					if format == FormatYAML {
						content = tc.yaml
					}
					path, _, err := layout.read(t, t.TempDir(), format, content)
					require.Error(t, err)

					assert.Equal(t, path+": "+tc.want, err.Error())
					var configErr *ConfigError
					require.ErrorAs(t, err, &configErr)
					assert.Equal(t, path, configErr.Path)
					assert.Equal(t, []string{tc.want}, schemaProblems(t, err))
				})
			}
		}
	}
}

func TestSchemaAcceptsConfigFiles(t *testing.T) {
	const schemaURL = "https://raw.githubusercontent.com/daFish/gogo-meta/v1.0.0/internal/config/gogo.schema.json"
	documents := map[ConfigFormat]string{
		FormatJSON: `{
  "$schema": "` + schemaURL + `",
  "projects": {"libs/any-name": "git@example.com:org/a.git", "2024": "git@example.com:org/b.git"},
  "groups": {"my group": ["libs/any-name", "2024"]},
  "commands": {
    "build": "make",
    "release.all": {"cmd": "make release", "parallel": true, "concurrency": 2, "groups": ["my group"]}
  }
}`,
		FormatYAML: `$schema: ` + schemaURL + `
projects:
  libs/any-name: git@example.com:org/a.git
  2024: git@example.com:org/b.git
groups:
  my group: [libs/any-name, "2024"]
commands:
  build: make
  release.all:
    cmd: make release
    parallel: true
    concurrency: 2
    groups: [my group]
`,
	}

	for format, content := range documents {
		for _, layout := range configLayouts {
			t.Run(fmt.Sprintf("%s/%s", format, layout.name), func(t *testing.T) {
				_, result, err := layout.read(t, t.TempDir(), format, content)
				require.NoError(t, err)

				cfg := result.Config
				assert.Equal(t, "git@example.com:org/b.git", cfg.Projects["2024"])
				assert.Equal(t, []string{"libs/any-name", "2024"}, cfg.Groups["my group"])
				assert.Equal(t, CommandConfig{Cmd: "make"}, cfg.Commands["build"])
				assert.Equal(t, "make release", cfg.Commands["release.all"].Cmd)
				require.NotNil(t, cfg.Commands["release.all"].Concurrency)
				assert.Equal(t, 2, *cfg.Commands["release.all"].Concurrency)
			})
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}

// TestSchemaKeepsWhatLoadedBefore pins configs that loaded before the schema
// existed and must keep loading with the same result: null for an absent
// value, keys that YAML reads as numbers or booleans, and integers that JSON
// Schema and yaml.v3 both accept although they are written as floats.
func TestSchemaKeepsWhatLoadedBefore(t *testing.T) {
	defaults := func(cfg MetaConfig) MetaConfig {
		if cfg.Projects == nil {
			cfg.Projects = map[string]string{}
		}
		if cfg.Ignore == nil {
			cfg.Ignore = append([]string{}, DefaultIgnore...)
		}
		return cfg
	}
	empty := defaults(MetaConfig{})

	cases := []struct {
		name    string
		format  ConfigFormat
		content string
		want    MetaConfig
	}{
		{"empty YAML file", FormatYAML, "", empty},
		{"YAML document marker only", FormatYAML, "---\n", empty},
		{"YAML comment only", FormatYAML, "# nothing yet\n", empty},
		{"YAML null document", FormatYAML, "~\n", empty},
		{"JSON null document", FormatJSON, "null", empty},
		{"YAML keys without values", FormatYAML, "$schema:\nprojects:\nignore:\ngroups:\ncommands:\n", empty},
		{"JSON null values", FormatJSON, `{"$schema":null,"projects":null,"ignore":null,"groups":null,"commands":null}`, empty},
		{
			"YAML non-string project paths keep their spelling", FormatYAML,
			"projects:\n  2024: u1\n  true: u2\n  1.0: u3\n  0x10: u4\n",
			defaults(MetaConfig{Projects: map[string]string{"2024": "u1", "true": "u2", "1.0": "u3", "0x10": "u4"}}),
		},
		{"YAML null project path is dropped", FormatYAML, "projects:\n  ~: url\n", empty},
		{"YAML null top-level key is dropped", FormatYAML, "projects: {}\n~: dropped\n", empty},
		{
			"YAML null key in a command is dropped", FormatYAML, "commands:\n  b: {cmd: x, ~: y}\n",
			defaults(MetaConfig{Commands: map[string]CommandConfig{"b": {Cmd: "x"}}}),
		},
		{
			"YAML null project URL", FormatYAML, "projects:\n  c:\n",
			defaults(MetaConfig{Projects: map[string]string{"c": ""}}),
		},
		{
			"JSON null project URL", FormatJSON, `{"projects":{"a":null}}`,
			defaults(MetaConfig{Projects: map[string]string{"a": ""}}),
		},
		{
			"YAML nulls in ignore", FormatYAML, "ignore: [a, ~]\n",
			defaults(MetaConfig{Ignore: []string{"a"}}),
		},
		{
			"YAML numeric group names and quoted members", FormatYAML,
			"projects: {2023: u}\ngroups:\n  2024: [\"2023\", ~]\n",
			defaults(MetaConfig{Projects: map[string]string{"2023": "u"}, Groups: map[string][]string{"2024": {"2023"}}}),
		},
		{
			"YAML nulls in command lists", FormatYAML,
			"commands:\n  b: {cmd: x, includeOnly: [~, a], groups: [g, ~]}\n",
			defaults(MetaConfig{Commands: map[string]CommandConfig{"b": {Cmd: "x", IncludeOnly: []string{"a"}, Groups: []string{"g"}}}}),
		},
		{
			"YAML integers written as floats or in hex", FormatYAML,
			"commands:\n  a: {cmd: x, concurrency: 2.0}\n  c: {cmd: x, concurrency: 0x02}\n  d: {cmd: x, concurrency: 1e3}\n",
			defaults(MetaConfig{Commands: map[string]CommandConfig{
				"a": {Cmd: "x", Concurrency: ptr(2)},
				"c": {Cmd: "x", Concurrency: ptr(2)},
				"d": {Cmd: "x", Concurrency: ptr(1000)},
			}}),
		},
		{
			"YAML largest concurrency", FormatYAML, "commands:\n  b: {cmd: x, concurrency: 9223372036854775807}\n",
			defaults(MetaConfig{Commands: map[string]CommandConfig{"b": {Cmd: "x", Concurrency: ptr(math.MaxInt)}}}),
		},
		{
			"JSON largest concurrency", FormatJSON, `{"commands":{"b":{"cmd":"x","concurrency":9223372036854775807}}}`,
			defaults(MetaConfig{Commands: map[string]CommandConfig{"b": {Cmd: "x", Concurrency: ptr(math.MaxInt)}}}),
		},
		{
			"YAML and JSON nulls in command fields", FormatJSON,
			`{"commands":{"b":{"cmd":"x","description":null,"parallel":null,"concurrency":null,"groups":null,"includeOnly":[null],"excludeOnly":null,"includePattern":null,"excludePattern":null}}}`,
			defaults(MetaConfig{Commands: map[string]CommandConfig{"b": {Cmd: "x", IncludeOnly: []string{""}}}}),
		},
		{
			"YAML anchors and merge keys", FormatYAML,
			"commands:\n  base: &base {cmd: x, parallel: true}\n  limits: &limits {cmd: y, concurrency: 3}\n  test:\n    <<: [*base, *limits]\n    description: merged\n",
			defaults(MetaConfig{Commands: map[string]CommandConfig{
				"base":   {Cmd: "x", Parallel: ptr(true)},
				"limits": {Cmd: "y", Concurrency: ptr(3)},
				"test":   {Cmd: "x", Description: "merged", Parallel: ptr(true), Concurrency: ptr(3)},
			}}),
		},
		{
			"YAML merge list: the earlier map wins", FormatYAML,
			"projects: {}\ncommands:\n  a: &a {cmd: x, concurrency: 2}\n  t:\n    <<: [*a, {concurrency: 0}]\n",
			defaults(MetaConfig{Commands: map[string]CommandConfig{
				"a": {Cmd: "x", Concurrency: ptr(2)},
				"t": {Cmd: "x", Concurrency: ptr(2)},
			}}),
		},
		{
			"YAML timestamp and binary scalars as strings", FormatYAML,
			"projects: {a: 2001-12-14, b: !!binary aGVsbG8=}\n",
			defaults(MetaConfig{Projects: map[string]string{"a": "2001-12-14", "b": "hello"}}),
		},
		{
			"YAML documents after the first are ignored", FormatYAML,
			"projects: {}\n---\nprojects: {a: b}\nunknown: key\n", empty,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseAndValidate(tc.content, tc.format)
			require.NoError(t, err)
			assert.Equal(t, tc.want, *cfg)
		})
	}
}

// TestSchemaRejectsKeysTheDecoderUsedToIgnore pins the configs that loaded
// before the schema only because a key was silently ignored, or, in JSON,
// matched to a field regardless of case.
func TestSchemaRejectsKeysTheDecoderUsedToIgnore(t *testing.T) {
	cases := []struct {
		name    string
		format  ConfigFormat
		content string
		want    []string
	}{
		{
			"JSON key in another case", FormatJSON, `{"Projects":{"a":"u"}}`,
			[]string{`unknown key "Projects" (allowed: $schema, projects, ignore, groups, commands)`},
		},
		{
			"JSON command key in another case", FormatJSON, `{"commands":{"b":{"cmd":"x","Parallel":true}}}`,
			[]string{`commands.b: unknown key "Parallel" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`},
		},
		{
			"YAML numeric top-level key", FormatYAML, "projects: {}\n2024: x\n",
			[]string{`unknown key "2024" (allowed: $schema, projects, ignore, groups, commands)`},
		},
		{
			"YAML anchor holder key", FormatYAML, "x-defaults: &d {cmd: a}\ncommands:\n  t:\n    <<: *d\n",
			[]string{`unknown key "x-defaults" (allowed: $schema, projects, ignore, groups, commands)`},
		},
		{
			"YAML unknown key merged into a command", FormatYAML, "commands:\n  base: &b {cmd: x, parallell: true}\n  t:\n    <<: *b\n",
			[]string{
				`commands.base: unknown key "parallell" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`,
				`commands.t: unknown key "parallell" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAndValidate(tc.content, tc.format)
			assert.Equal(t, tc.want, schemaProblems(t, err))
		})
	}
}

// TestSchemaRulesAreTheSameInBothFormats pins that YAML values are checked
// with the type YAML gives them, as in JSON: an unquoted number is no string,
// yes/no/on/off are no booleans and a fraction is no integer.
func TestSchemaRulesAreTheSameInBothFormats(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		json string
		want []string
	}{
		{
			"numbers and booleans where strings are wanted",
			"$schema: 5\nprojects: {a: 2024}\nignore: [true]\ngroups: {g: [2024]}\n",
			`{"$schema":5,"projects":{"a":2024},"ignore":[true],"groups":{"g":[2024]}}`,
			[]string{
				`$schema: must be a string (got 5)`,
				`projects.a: must be a string (got 2024)`,
				`ignore[0]: must be a string (got true)`,
				`groups.g[0]: must be a string (got 2024)`,
			},
		},
		{
			"a number as a command",
			"commands: {build: 42, test: {cmd: 42}}\n",
			`{"commands":{"build":42,"test":{"cmd":42}}}`,
			[]string{
				`commands.build: must be a string or a map (got 42)`,
				`commands.test.cmd: must be a string (got 42)`,
			},
		},
		{
			"YAML 1.1 words are no booleans",
			"commands: {b: {cmd: x, parallel: yes}}\n",
			`{"commands":{"b":{"cmd":"x","parallel":"yes"}}}`,
			[]string{`commands.b.parallel: must be a boolean (got "yes")`},
		},
		{
			"a fraction is no integer and is reported once",
			"commands: {b: {cmd: x, concurrency: 2.5}}\n",
			`{"commands":{"b":{"cmd":"x","concurrency":2.5}}}`,
			[]string{`commands.b.concurrency: must be an integer (got 2.5)`},
		},
	}

	for _, tc := range cases {
		for format, content := range map[ConfigFormat]string{FormatYAML: tc.yaml, FormatJSON: tc.json} {
			t.Run(fmt.Sprintf("%s/%s", format, tc.name), func(t *testing.T) {
				_, err := parseContent([]byte(content), format)
				assert.ElementsMatch(t, tc.want, schemaProblems(t, err))
			})
		}
	}
}

func TestSchemaErrorMessages(t *testing.T) {
	cases := []struct {
		name    string
		format  ConfigFormat
		content string
		want    []string
	}{
		{
			"every problem, ordered by location", FormatJSON,
			`{"commands":{"b":{"cmd":"x","parallell":true},"a":{"cmd":""}},"ignores":[],"group":{}}`,
			[]string{
				`unknown key "group" (allowed: $schema, projects, ignore, groups, commands)`,
				`unknown key "ignores" (allowed: $schema, projects, ignore, groups, commands)`,
				`commands.a.cmd: must not be empty`,
				`commands.b: unknown key "parallell" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`,
			},
		},
		{"a document that is not a map", FormatJSON, `["api"]`, []string{`must be a map (got a list)`}},
		{"a YAML document that is not a map", FormatYAML, "api\n", []string{`must be a map (got "api")`}},
		{"list items by index", FormatYAML, "ignore: [a, [b]]\n", []string{`ignore[1]: must be a string (got a list)`}},
		{
			"keys that contain dots are quoted", FormatJSON,
			`{"commands":{"build.all":{"cmd":"make","oops":1}}}`,
			[]string{`commands."build.all": unknown key "oops" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`},
		},
		{"either of several types", FormatYAML, "commands:\n  b: [make]\n", []string{`commands.b: must be a string or a map (got a list)`}},
		{"null is not offered as a fix", FormatYAML, "commands:\n  b: {cmd: x, parallel: maybe}\n", []string{`commands.b.parallel: must be a boolean (got "maybe")`}},
		{"null where a value is needed", FormatYAML, "commands:\n  b:\n", []string{`commands.b: must be a string or a map (got null)`}},
		{"JSON is strict about types", FormatJSON, `{"commands":{"b":{"cmd":"x","parallel":"yes","concurrency":2.5}}}`, []string{
			`commands.b.concurrency: must be an integer (got 2.5)`,
			`commands.b.parallel: must be a boolean (got "yes")`,
		}},
		{"YAML quoted true is no boolean", FormatYAML, "commands:\n  b: {cmd: x, parallel: \"true\"}\n", []string{`commands.b.parallel: must be a boolean (got "true")`}},
		{"YAML fraction is no integer", FormatYAML, "commands:\n  b: {cmd: x, concurrency: 0.9}\n", []string{`commands.b.concurrency: must be an integer (got 0.9)`}},
		{"YAML minimum", FormatYAML, "commands:\n  b: {cmd: x, concurrency: -1}\n", []string{`commands.b.concurrency: must be >= 1 (got -1)`}},
		{
			"YAML own key overrides a merged key", FormatYAML,
			"projects: {}\ncommands:\n  a: &b {cmd: x, concurrency: 2}\n  t: {<<: *b, concurrency: 0}\n",
			[]string{`commands.t.concurrency: must be >= 1 (got 0)`},
		},
		{"JSON numbers are quoted as written", FormatJSON, `{"commands":{"a":{"cmd":"x","concurrency":0.0000001},"b":{"cmd":"x","concurrency":-12345678901234567890}}}`, []string{
			`commands.a.concurrency: must be an integer (got 0.0000001)`,
			`commands.b.concurrency: must be >= 1 (got -12345678901234567890)`,
		}},
		{"JSON integer written as a float", FormatJSON, `{"commands":{"b":{"cmd":"x","concurrency":2.0}}}`, []string{`commands.b.concurrency: must be an integer (got 2.0)`}},
		{"JSON integer written as a float, next to another problem", FormatJSON, `{"bogus":1,"commands":{"b":{"cmd":"x","concurrency":2.0}}}`, []string{
			`unknown key "bogus" (allowed: $schema, projects, ignore, groups, commands)`,
			`commands.b.concurrency: must be an integer (got 2.0)`,
		}},
		{"JSON integer out of range", FormatJSON, `{"commands":{"b":{"cmd":"x","concurrency":99999999999999999999}}}`, []string{`commands.b.concurrency: must be <= 9223372036854775807 (got 99999999999999999999)`}},
		{"YAML integer out of range", FormatYAML, "commands:\n  b: {cmd: x, concurrency: 9223372036854775808}\n", []string{`commands.b.concurrency: must be <= 9223372036854775807 (got 9223372036854775808)`}},
		{"YAML float out of range", FormatYAML, "commands:\n  b: {cmd: x, concurrency: 1e30}\n", []string{`commands.b.concurrency: must be <= 9223372036854775807 (got 1e+30)`}},
		{"list indexes in numeric order", FormatJSON, `{"ignore":["a","b",2,"c","d","e","f","g","h","i",10]}`, []string{
			`ignore[2]: must be a string (got 2)`,
			`ignore[10]: must be a string (got 10)`,
		}},
		{"keys with a colon or a newline are quoted", FormatJSON, `{"projects":{"a\nb":9,"a:b":1}}`, []string{
			`projects."a\nb": must be a string (got 9)`,
			`projects."a:b": must be a string (got 1)`,
		}},
		{
			"a command name with a newline stays on one line", FormatYAML,
			"projects: {}\ncommands:\n  \"a\\nb\": {cmd: x, bad: 1}\n",
			[]string{`commands."a\nb": unknown key "bad" (allowed: cmd, description, parallel, concurrency, groups, includeOnly, excludeOnly, includePattern, excludePattern)`},
		},
		{
			"JSON duplicate top-level key hiding an invalid command", FormatJSON,
			`{"projects":{"api":"u"},"commands":{"b":{"cmd":"echo hi","concurrency":0,"parallel":true}},"commands":{}}`,
			[]string{`duplicate key "commands"`},
		},
		{
			"JSON duplicate top-level key hiding a missing cmd and an unknown key", FormatJSON,
			`{"projects":{},"commands":{"b":{"parallel":true,"parallell":true}},"commands":{"c":"echo y"}}`,
			[]string{`duplicate key "commands"`},
		},
		{"JSON duplicate command", FormatJSON, `{"commands":{"b":{"cmd":"x","bogus":1},"b":"y"}}`, []string{`commands: duplicate key "b"`}},
		{"JSON key repeated three times is reported once", FormatJSON, `{"projects":{"a":"x","a":"y","a":"z"}}`, []string{`projects: duplicate key "a"`}},
		{"YAML infinity is no integer", FormatYAML, "commands:\n  b: {cmd: x, concurrency: .inf}\n", []string{`commands.b.concurrency: must be an integer (got ".inf")`}},
		{"empty string-form command", FormatJSON, `{"commands":{"b":""}}`, []string{`commands.b: must not be empty`}},
		{"group members are a list", FormatYAML, "groups:\n  backend:\n", []string{`groups.backend: must be a list (got null)`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseContent([]byte(tc.content), tc.format)
			problems := schemaProblems(t, err)
			assert.Equal(t, tc.want, problems)
			for _, problem := range problems {
				for _, leak := range []string{"config.", "map[", "Alias", "#/", "jsonschema", "additionalProperties", "got object", "want ", "!!", "unmarshal"} {
					assert.NotContains(t, problem, leak)
				}
			}
		})
	}
}

// TestValidateKeepsWhatTheSchemaCannotGuarantee pins the rules that stay in
// Validate. A group left empty is one of them: yaml.v3 drops null list items
// only after the schema has counted them, so the schema cannot see it.
func TestSchemaProblemOrderIsStable(t *testing.T) {
	content := []byte(`{"projects":{"1a":1,"10":1,"2":1,"b":1,"01":1}}`)
	want := []string{
		`projects.01: must be a string (got 1)`,
		`projects.2: must be a string (got 1)`,
		`projects.10: must be a string (got 1)`,
		`projects.1a: must be a string (got 1)`,
		`projects.b: must be a string (got 1)`,
	}
	// The problems come from map iteration, so repeat to catch any order that
	// depends on it.
	for range 20 {
		_, err := parseContent(content, FormatJSON)
		require.Equal(t, want, schemaProblems(t, err))
	}
}

func TestValidateKeepsWhatTheSchemaCannotGuarantee(t *testing.T) {
	cases := []struct {
		name    string
		format  ConfigFormat
		content string
		want    string
	}{
		{"YAML group of null members", FormatYAML, "projects: {a: u}\ngroups:\n  g: [~]\n", `group "g": must contain at least one project`},
		{"JSON group without members", FormatJSON, `{"projects":{"a":"u"},"groups":{"g":[]}}`, `group "g": must contain at least one project`},
		{"blank group member", FormatJSON, `{"projects":{"a":"u"},"groups":{"g":["a",null]}}`, `group "g": project paths must not be empty`},
		{"blank group name of a command", FormatJSON, `{"commands":{"b":{"cmd":"x","groups":[" "]}}}`, `command "b": group names must not be empty`},
		{"unsafe project path", FormatYAML, "projects:\n  ../evil: u\n", `invalid project path "../evil": must be relative and stay within the repository`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseContent([]byte(tc.content), tc.format)
			require.NoError(t, err, "the schema accepts it")

			_, err = parseAndValidate(tc.content, tc.format)
			require.Error(t, err)
			assert.Equal(t, tc.want, err.Error())
		})
	}
}

func TestSchemaErrorTranslationOfOtherKeywords(t *testing.T) {
	schema, err := compileConfigSchema([]byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "execution": {
      "type": "object",
      "properties": {
        "mode": {"enum": ["parallel", "sequential"]},
        "limit": {"type": "integer", "maximum": 8},
        "name": {"type": "string", "minLength": 3, "pattern": "^[a-z]+$"}
      }
    }
  }
}`))
	require.NoError(t, err)

	document := map[string]any{"execution": map[string]any{"mode": "parallell", "limit": float64(9), "name": "A"}}
	err = schema.validate(document)
	assert.Equal(t, []string{
		`execution.limit: must be <= 8 (got 9)`,
		`execution.mode: must be one of "parallel", "sequential" (got "parallell")`,
		`execution.name: is not valid`,
		`execution.name: must be at least 3 characters long`,
	}, schemaProblems(t, err))
}

func TestYAMLAliasesAreBounded(t *testing.T) {
	t.Run("an anchor that contains itself", func(t *testing.T) {
		_, err := parseContent([]byte("projects: {}\ncommands: &x {t: *x}\n"), FormatYAML)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchor 'x' value contains itself")
	})

	t.Run("exponential alias expansion", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("a: &a [lol, lol, lol, lol, lol, lol, lol, lol, lol]\n")
		previous := "a"
		for _, name := range []string{"b", "c", "d", "e", "f", "g", "h", "i"} {
			fmt.Fprintf(&b, "%s: &%s [%s]\n", name, name, strings.TrimSuffix(strings.Repeat("*"+previous+", ", 9), ", "))
			previous = name
		}
		_, err := parseContent([]byte(b.String()), FormatYAML)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "excessive aliasing")
	})

	listOf := func(count int) string {
		items := make([]string, count)
		for i := range items {
			items[i] = fmt.Sprintf("p%d", i)
		}
		return "[" + strings.Join(items, ", ") + "]"
	}

	t.Run("many commands aliasing one large command", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("projects: {}\ncommands:\n  base: &b {cmd: x, includeOnly: " + listOf(200) + "}\n")
		for i := range 200 {
			fmt.Fprintf(&b, "  c%d: *b\n", i)
		}
		cfg, err := parseAndValidate(b.String(), FormatYAML)
		require.NoError(t, err)
		assert.Len(t, cfg.Commands["c199"].IncludeOnly, 200)
	})

	t.Run("many groups aliasing one large group", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("projects:\n")
		for i := range 600 {
			fmt.Fprintf(&b, "  p%d: u\n", i)
		}
		b.WriteString("groups:\n  all: &all " + listOf(600) + "\n")
		for i := range 300 {
			fmt.Fprintf(&b, "  g%d: *all\n", i)
		}
		cfg, err := parseAndValidate(b.String(), FormatYAML)
		require.NoError(t, err)
		assert.Len(t, cfg.Groups["g299"], 600)
	})

	t.Run("aliases across commands count for the whole document", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("projects: {}\nignore: &list " + listOf(900) + "\ncommands:\n")
		for i := range 450 {
			fmt.Fprintf(&b, "  c%d: {cmd: x, includeOnly: *list}\n", i)
		}
		_, err := parseContent([]byte(b.String()), FormatYAML)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "excessive aliasing")
	})
}

func TestYAMLKeysMustBeScalars(t *testing.T) {
	for name, content := range map[string]string{
		"a list as a key":         "projects: {}\n? [a]\n: b\n",
		"a map as a project path": "projects:\n  ? {a: b}\n  : u\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseContent([]byte(content), FormatYAML)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid map key")
		})
	}
}

func TestSchemaKeyRoundTrips(t *testing.T) {
	const schemaURL = "https://raw.githubusercontent.com/daFish/gogo-meta/v1.0.0/internal/config/gogo.schema.json"
	documents := map[ConfigFormat]string{
		FormatJSON: `{"projects":{"a":"urlA"},"$schema":"` + schemaURL + `"}`,
		FormatYAML: "projects:\n  a: urlA\n$schema: " + schemaURL + "\n",
	}
	firstLines := map[ConfigFormat]string{
		FormatJSON: `  "$schema": "` + schemaURL + `",`,
		FormatYAML: "$schema: " + schemaURL,
	}

	for format, content := range documents {
		t.Run(string(format), func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, FilenameForFormat(format)), content)

			result, err := ReadMetaConfig(dir, []string{})
			require.NoError(t, err)
			assert.Equal(t, schemaURL, result.Config.Schema)

			// The project create and import tests cover the read-modify-write cycle
			// end to end; passing the read config straight back in here trips
			// gosec's taint analysis of the file write.
			read := MetaConfig{Schema: result.Config.Schema, Projects: map[string]string{"a": "urlA"}}
			require.NoError(t, WriteMetaConfig(dir, AddProject(read, "b", "urlB"), format))

			written, err := os.ReadFile(filepath.Join(dir, FilenameForFormat(format)))
			require.NoError(t, err)
			lines := strings.Split(string(written), "\n")
			if format == FormatJSON {
				lines = lines[1:]
			}
			assert.Equal(t, firstLines[format], lines[0], "$schema is written first")

			reread, err := ReadMetaConfig(dir, []string{})
			require.NoError(t, err)
			assert.Equal(t, schemaURL, reread.Config.Schema)
			assert.Equal(t, map[string]string{"a": "urlA", "b": "urlB"}, reread.Config.Projects)
		})
	}

	t.Run("project changes and merges keep it", func(t *testing.T) {
		cfg := MetaConfig{Schema: schemaURL, Projects: map[string]string{"a": "urlA"}}
		assert.Equal(t, schemaURL, AddProject(cfg, "b", "urlB").Schema)
		assert.Equal(t, schemaURL, RemoveProject(cfg, "a").Schema)
		assert.Equal(t, schemaURL, MergeConfigs(cfg, MetaConfig{Schema: "other", Projects: map[string]string{}}).Schema)
	})
}

func TestWriteMetaConfigRefusesWhatItCouldNotRead(t *testing.T) {
	dir := t.TempDir()
	cfg := MetaConfig{
		Projects: map[string]string{"a": "urlA"},
		Commands: map[string]CommandConfig{"build": {Cmd: "make", Concurrency: ptr(0)}},
	}

	err := WriteMetaConfig(dir, cfg, FormatJSON)
	assert.Equal(t, []string{"commands.build.concurrency: must be >= 1 (got 0)"}, schemaProblems(t, err))
	assert.NoFileExists(t, filepath.Join(dir, ".gogo"))
}
