package build

import (
	"fmt"

	"github.com/goccy/go-yaml/ast"
)

var errModuleTypeRequired = fmt.Errorf("module type is required")

func (m *Module) UnmarshalYAML(n ast.Node) error {
	env, raw, err := decodeYamlEnvelope(n)
	if err != nil {
		return err
	}
	return assembleModule(env, raw, m)
}

func (m *Module) UnmarshalJSON(data []byte) error {
	env, raw, err := decodeJsonEnvelope(data)
	if err != nil {
		return err
	}
	return assembleModule(env, raw, m)
}

// assembleModule builds a module from its decoded envelope and raw body.
func assembleModule(env moduleEnvelope, raw rawBody, m *Module) error {
	env.applyTo(m)
	if m.Type == "" {
		return errModuleTypeRequired
	}

	body, err := NewBody(m.Type)
	if err != nil {
		return err
	}
	if err := raw.decode(body); err != nil {
		return fmt.Errorf("failed to decode module %s: %w", m.GetLabel(), err)
	}
	m.Body = body
	return nil
}
