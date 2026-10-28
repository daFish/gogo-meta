package config

import (
	"bytes"
	"cmp"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v3"
)

//go:embed gogo.schema.json
var schemaJSON []byte

const schemaResource = "urn:gogo:config-schema"

// SchemaError lists every way a config document violates the config schema,
// one problem per entry, phrased "<location>: <what is wrong>" or, at the top
// level, just "<what is wrong>".
type SchemaError struct {
	Problems []string
}

func (e *SchemaError) Error() string {
	return strings.Join(e.Problems, "\n")
}

type configSchema struct {
	validator *jsonschema.Schema
	root      *schemaNode
}

var loadConfigSchema = sync.OnceValues(func() (*configSchema, error) {
	return compileConfigSchema(schemaJSON)
})

func compileConfigSchema(data []byte) (*configSchema, error) {
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid config schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaResource, document); err != nil {
		return nil, fmt.Errorf("invalid config schema: %w", err)
	}
	validator, err := compiler.Compile(schemaResource)
	if err != nil {
		return nil, fmt.Errorf("invalid config schema: %w", err)
	}
	var root schemaNode
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid config schema: %w", err)
	}
	return &configSchema{validator: validator, root: &root}, nil
}

// validateConfigDocument checks raw config content against the config schema
// before it is decoded into a MetaConfig. Syntax errors are returned as the
// decoders report them; schema violations as a *SchemaError.
func validateConfigDocument(content []byte, format ConfigFormat) error {
	document, problems, err := decodeDocument(content, format)
	if err != nil {
		return err
	}
	if document == nil {
		document = map[string]any{}
	}
	schema, err := loadConfigSchema()
	if err != nil {
		return err
	}
	return schema.validate(document, problems...)
}

// validate checks a document against the schema. Problems found while
// decoding the document, such as repeated keys, are reported along with the
// schema's own.
func (s *configSchema) validate(document any, problems ...schemaProblem) error {
	violations, err := s.violations(document)
	if err != nil {
		return err
	}
	for _, violation := range violations {
		problems = append(problems, s.translate(violation, document)...)
	}
	problems = append(problems, s.root.integerLiterals(document, nil)...)
	if len(problems) == 0 {
		return nil
	}

	slices.SortFunc(problems, compareProblems)
	messages := make([]string, len(problems))
	for i, problem := range problems {
		messages[i] = problem.render(document)
	}
	return &SchemaError{Problems: slices.Compact(messages)}
}

// violations returns the individual failed keywords of a validation run.
func (s *configSchema) violations(document any) ([]*jsonschema.ValidationError, error) {
	err := s.validator.Validate(document)
	if err == nil {
		return nil, nil
	}
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return nil, err
	}
	return leafErrors(validationErr), nil
}

func leafErrors(err *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(err.Causes) == 0 {
		return []*jsonschema.ValidationError{err}
	}
	var leaves []*jsonschema.ValidationError
	for _, cause := range err.Causes {
		leaves = append(leaves, leafErrors(cause)...)
	}
	return leaves
}

// schemaProblem is one thing wrong with a config document: what is wrong, and
// where, as the path of keys and list indexes leading to the value.
type schemaProblem struct {
	tokens  []string
	message string
}

func (p schemaProblem) render(document any) string {
	location := formatLocation(document, p.tokens)
	if location == "" {
		return p.message
	}
	return location + ": " + p.message
}

// compareProblems orders problems by location, list indexes by number, and
// then by message.
func compareProblems(a, b schemaProblem) int {
	if c := slices.CompareFunc(a.tokens, b.tokens, compareTokens); c != 0 {
		return c
	}
	return strings.Compare(a.message, b.message)
}

// compareTokens puts numeric tokens first, in numeric order, and all other
// tokens after them in string order.
func compareTokens(a, b string) int {
	x, errA := strconv.Atoi(a)
	y, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return cmp.Or(cmp.Compare(x, y), strings.Compare(a, b))
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}

// translate turns one validation error of the schema library into problems
// worded for gogo users: YAML- and JSON-neutral type names and the allowed keys
// next to an unknown one. The location becomes a dotted path when rendered.
func (s *configSchema) translate(err *jsonschema.ValidationError, document any) []schemaProblem {
	tokens := err.InstanceLocation
	value := valueAt(document, tokens)
	problem := func(message string) []schemaProblem {
		return []schemaProblem{{tokens: tokens, message: message}}
	}

	switch k := err.ErrorKind.(type) {
	case *kind.AdditionalProperties:
		allowed := s.root.lookup(schemaPointer(err.SchemaURL)).propertyNames()
		var problems []schemaProblem
		for _, name := range slices.Sorted(slices.Values(k.Properties)) {
			message := fmt.Sprintf("unknown key %q", name)
			if len(allowed) > 0 {
				message += fmt.Sprintf(" (allowed: %s)", strings.Join(allowed, ", "))
			}
			problems = append(problems, schemaProblem{tokens: tokens, message: message})
		}
		return problems
	case *kind.Required:
		var problems []schemaProblem
		for _, name := range k.Missing {
			problems = append(problems, schemaProblem{tokens: tokens, message: fmt.Sprintf("missing required key %q", name)})
		}
		return problems
	case *kind.Type:
		return problem(fmt.Sprintf("must be %s (got %s)", describeTypes(k.Want), describeValue(value)))
	case *kind.Enum:
		return problem(fmt.Sprintf("must be one of %s (got %s)", describeEnum(k.Want), describeValue(value)))
	case *kind.Minimum:
		return problem(fmt.Sprintf("must be >= %s (got %s)", formatRat(k.Want), describeValue(value)))
	case *kind.Maximum:
		return problem(fmt.Sprintf("must be <= %s (got %s)", formatRat(k.Want), describeValue(value)))
	case *kind.MinLength:
		if k.Want == 1 {
			return problem("must not be empty")
		}
		return problem(fmt.Sprintf("must be at least %d characters long", k.Want))
	default:
		return problem("is not valid")
	}
}

var typeNames = map[string]string{
	"string":  "a string",
	"integer": "an integer",
	"number":  "a number",
	"boolean": "a boolean",
	"array":   "a list",
	"object":  "a map",
	"null":    "null",
}

func describeTypes(want []string) string {
	var names []string
	for _, t := range want {
		if t != "null" || len(want) == 1 {
			names = append(names, typeNames[t])
		}
	}
	return strings.Join(names, " or ")
}

// describeEnum lists the allowed values of an enum. Like describeTypes, it
// does not offer null as a fix unless null is the only value.
func describeEnum(want []any) string {
	var values []string
	for _, value := range want {
		if value != nil || len(want) == 1 {
			values = append(values, describeValue(value))
		}
	}
	return strings.Join(values, ", ")
}

func describeValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(v)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case json.Number:
		return v.String()
	case []any:
		return typeNames["array"]
	case map[string]any:
		return typeNames["object"]
	default:
		return fmt.Sprint(v)
	}
}

func formatRat(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// formatLocation renders an instance location as a dotted path, with list
// indexes in brackets and keys quoted when they would read ambiguously.
func formatLocation(document any, tokens []string) string {
	var b strings.Builder
	current := document
	for _, token := range tokens {
		switch node := current.(type) {
		case []any:
			b.WriteString("[" + token + "]")
			if index, err := strconv.Atoi(token); err == nil && index >= 0 && index < len(node) {
				current = node[index]
			}
		default:
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			if needsQuotes(token) {
				b.WriteString(strconv.Quote(token))
			} else {
				b.WriteString(token)
			}
			if object, ok := node.(map[string]any); ok {
				current = object[token]
			}
		}
	}
	return b.String()
}

// needsQuotes reports whether a key would read ambiguously in a dotted path,
// or would not stay on one line, unless quoted.
func needsQuotes(token string) bool {
	return token == "" || strings.ContainsAny(token, ".[]\"' :") ||
		strconv.Quote(token) != `"`+token+`"`
}

func valueAt(document any, tokens []string) any {
	current := document
	for _, token := range tokens {
		switch node := current.(type) {
		case map[string]any:
			current = node[token]
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return nil
			}
			current = node[index]
		default:
			return nil
		}
	}
	return current
}

// schemaPointer extracts the JSON pointer tokens from a schema location such
// as "urn:gogo:config-schema#/properties/commands/additionalProperties".
func schemaPointer(location string) []string {
	_, fragment, _ := strings.Cut(location, "#")
	fragment = strings.TrimPrefix(fragment, "/")
	if fragment == "" {
		return nil
	}
	tokens := strings.Split(fragment, "/")
	for i, token := range tokens {
		if unescaped, err := url.PathUnescape(token); err == nil {
			token = unescaped
		}
		tokens[i] = strings.NewReplacer("~1", "/", "~0", "~").Replace(token)
	}
	return tokens
}

// schemaNode is the part of the schema the error translation reads: the
// declared types and properties, in the order the schema lists them, and the
// subschemas reachable through properties, additionalProperties and items.
type schemaNode struct {
	Type                 schemaTypes      `json:"type"`
	Properties           schemaProperties `json:"properties"`
	AdditionalProperties *schemaNode      `json:"additionalProperties"`
	Items                *schemaNode      `json:"items"`
}

// schemaTypes is the type keyword, which names one type or a list of them.
type schemaTypes []string

func (t *schemaTypes) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*t = schemaTypes{single}
		return nil
	}
	return json.Unmarshal(data, (*[]string)(t))
}

func (n *schemaNode) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("true")) || bytes.Equal(data, []byte("false")) {
		return nil
	}
	type plain schemaNode
	return json.Unmarshal(data, (*plain)(n))
}

func (n *schemaNode) lookup(pointer []string) *schemaNode {
	current := n
	for i := 0; current != nil && i < len(pointer); i++ {
		switch pointer[i] {
		case "properties":
			if i+1 >= len(pointer) {
				return nil
			}
			i++
			current = current.Properties.nodes[pointer[i]]
		case "additionalProperties":
			current = current.AdditionalProperties
		case "items":
			current = current.Items
		default:
			return nil
		}
	}
	return current
}

// child returns the subschema that applies to a key of an object or an item
// of a list.
func (n *schemaNode) child(container any, token string) *schemaNode {
	if _, isList := container.([]any); isList {
		return n.Items
	}
	if node, ok := n.Properties.nodes[token]; ok {
		return node
	}
	return n.AdditionalProperties
}

// integerLiterals reports JSON numbers written with a fraction or an exponent
// where the schema wants an integer. JSON Schema counts 2.0 and 1e3 as
// integers, but the decoder into MetaConfig cannot read them into an int.
func (n *schemaNode) integerLiterals(value any, tokens []string) []schemaProblem {
	if n == nil {
		return nil
	}
	var problems []schemaProblem
	switch v := value.(type) {
	case json.Number:
		if slices.Contains(n.Type, "integer") && strings.ContainsAny(v.String(), ".eE") {
			problems = append(problems, schemaProblem{
				tokens:  slices.Clone(tokens),
				message: fmt.Sprintf("must be an integer (got %s)", v),
			})
		}
	case map[string]any:
		for key, item := range v {
			problems = append(problems, n.child(v, key).integerLiterals(item, append(tokens, key))...)
		}
	case []any:
		for i, item := range v {
			token := strconv.Itoa(i)
			problems = append(problems, n.child(v, token).integerLiterals(item, append(tokens, token))...)
		}
	}
	return problems
}

func (n *schemaNode) propertyNames() []string {
	if n == nil {
		return nil
	}
	return n.Properties.names
}

type schemaProperties struct {
	names []string
	nodes map[string]*schemaNode
}

func (p *schemaProperties) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return err
	}
	p.nodes = make(map[string]*schemaNode)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			return fmt.Errorf("unexpected property name %v", token)
		}
		node := &schemaNode{}
		if err := decoder.Decode(node); err != nil {
			return err
		}
		p.names = append(p.names, name)
		p.nodes[name] = node
	}
	return nil
}

func decodeDocument(content []byte, format ConfigFormat) (any, []schemaProblem, error) {
	if format == FormatYAML {
		return decodeYAMLDocument(content)
	}
	return decodeJSONDocument(content)
}

// decodeJSONDocument converts JSON into the data model the schema validates.
// Numbers keep the spelling they were written in, so messages quote them as
// written. A key repeated in an object is a problem: the schema would see only
// its last value, while the decoder into MetaConfig merges all of them.
func decodeJSONDocument(content []byte) (any, []schemaProblem, error) {
	// Syntax errors keep the wording of json.Unmarshal.
	if err := json.Unmarshal(content, new(json.RawMessage)); err != nil {
		return nil, nil, err
	}
	reader := jsonReader{decoder: json.NewDecoder(bytes.NewReader(content))}
	reader.decoder.UseNumber()
	document, err := reader.value()
	if err != nil {
		return nil, nil, err
	}
	return document, reader.problems, nil
}

type jsonReader struct {
	decoder  *json.Decoder
	path     []string
	problems []schemaProblem
}

func (r *jsonReader) value() (any, error) {
	token, err := r.decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		return r.object()
	case json.Delim('['):
		return r.list()
	}
	return token, nil
}

func (r *jsonReader) object() (any, error) {
	object := make(map[string]any)
	for r.decoder.More() {
		token, err := r.decoder.Token()
		if err != nil {
			return nil, err
		}
		key, _ := token.(string)
		if _, repeated := object[key]; repeated {
			r.problems = append(r.problems, schemaProblem{
				tokens:  slices.Clone(r.path),
				message: fmt.Sprintf("duplicate key %q", key),
			})
		}
		value, err := r.member(key)
		if err != nil {
			return nil, err
		}
		object[key] = value
	}
	_, err := r.decoder.Token()
	return object, err
}

func (r *jsonReader) list() (any, error) {
	items := make([]any, 0)
	for r.decoder.More() {
		item, err := r.member(strconv.Itoa(len(items)))
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	_, err := r.decoder.Token()
	return items, err
}

func (r *jsonReader) member(token string) (any, error) {
	r.path = append(r.path, token)
	defer func() { r.path = r.path[:len(r.path)-1] }()
	return r.value()
}

// decodeYAMLDocument converts YAML into the JSON data model the schema
// validates. yaml.Unmarshal resolves aliases and merge keys within its own
// limits and rejects repeated keys. Scalars keep the type YAML gives them, so
// `2024` is a number and `yes` a string, exactly as an editor validating
// against the schema sees them.
func decodeYAMLDocument(content []byte) (any, []schemaProblem, error) {
	var document any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, nil, err
	}
	return jsonValue(document), nil, nil
}

// jsonValue turns what yaml.v3 decodes into an untyped value into its JSON
// counterpart. Keys become strings and null keys are dropped, as they are when
// decoding into a MetaConfig. Values JSON cannot express are spelled as YAML
// writes them.
func jsonValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			v[key] = jsonValue(item)
		}
		return v
	case map[any]any:
		object := make(map[string]any, len(v))
		for key, item := range v {
			if key != nil {
				object[fmt.Sprint(key)] = jsonValue(item)
			}
		}
		return object
	case []any:
		for i, item := range v {
			v[i] = jsonValue(item)
		}
		return v
	case float64:
		switch {
		case math.IsNaN(v):
			return ".nan"
		case math.IsInf(v, 1):
			return ".inf"
		case math.IsInf(v, -1):
			return "-.inf"
		}
		return v
	case time.Time:
		return v.Format(time.RFC3339Nano)
	}
	return value
}
