package build

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// Codec decodes a build recipe document from one wire format, applying the same
// strict policy (unknown fields rejected) regardless of format.
type Codec interface {
	Name() string
	DecodeStrict(data []byte, v any) error
}

type yamlCodec struct{}

func (yamlCodec) Name() string { return "yaml" }

func (yamlCodec) DecodeStrict(data []byte, v any) error {
	return yaml.NewDecoder(bytes.NewReader(data), yaml.DisallowUnknownField()).Decode(v)
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) DecodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// codecForPath picks a codec by file extension.
func codecForPath(path string) Codec {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return jsonCodec{}
	}
	return yamlCodec{}
}

// isRecipeFile reports whether path is a supported recipe file.
func isRecipeFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yml", ".yaml", ".json":
		return true
	}
	return false
}

// moduleEnvelope holds the flat module fields shared by both formats.
type moduleEnvelope struct {
	Name   string            `yaml:"name" json:"name"`
	Type   string            `yaml:"type" json:"type"`
	Id     string            `yaml:"id" json:"id"`
	Env    map[string]string `yaml:"env" json:"env"`
	If     string            `yaml:"if" json:"if"`
	Output map[string]string `yaml:"output" json:"output"`
}

func (e moduleEnvelope) applyTo(m *Module) {
	m.Name = e.Name
	m.Type = e.Type
	m.Id = e.Id
	m.Env = e.Env
	m.If = e.If
	m.Output = e.Output
}

// rawBody is a captured, not-yet-typed module body; decode fills a typed Body.
type rawBody interface {
	decode(body Body) error
}

type yamlRawBody struct{ node ast.MappingNode }

func (b yamlRawBody) decode(body Body) error {
	node := b.node
	return yaml.NewDecoder(&node, yaml.DisallowUnknownField()).Decode(body)
}

type jsonRawBody struct{ raw json.RawMessage }

func (b jsonRawBody) decode(body Body) error {
	dec := json.NewDecoder(bytes.NewReader(b.raw))
	dec.DisallowUnknownFields()
	return dec.Decode(body)
}

// decodeYamlEnvelope splits a YAML module node into flat fields and a raw body.
func decodeYamlEnvelope(n ast.Node) (moduleEnvelope, rawBody, error) {
	var aux struct {
		Fields moduleEnvelope  `yaml:",inline"`
		Body   ast.MappingNode `yaml:"body"`
	}
	if err := yaml.NewDecoder(n, yaml.DisallowUnknownField()).Decode(&aux); err != nil {
		return moduleEnvelope{}, nil, err
	}
	return aux.Fields, yamlRawBody{node: aux.Body}, nil
}

// decodeJsonEnvelope splits a JSON module object into flat fields and a raw body.
func decodeJsonEnvelope(data []byte) (moduleEnvelope, rawBody, error) {
	var aux struct {
		moduleEnvelope
		Body json.RawMessage `json:"body"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&aux); err != nil {
		return moduleEnvelope{}, nil, err
	}
	return aux.moduleEnvelope, jsonRawBody{raw: aux.Body}, nil
}
