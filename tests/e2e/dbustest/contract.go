//go:build e2e

package dbustest

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5/introspect"
)

// InterfaceContract maps method names to readable introspection signatures.
type InterfaceContract map[string]string

// AssertContract introspects the client's object and verifies one complete interface.
func (c *Client) AssertContract(t testing.TB, interfaceName string, want InterfaceContract) {
	t.Helper()

	var raw string
	c.Request("org.freedesktop.DBus.Introspectable", "Introspect").Store(t, &raw)

	var node introspect.Node
	if err := xml.Unmarshal([]byte(raw), &node); err != nil {
		t.Fatalf("decode introspection XML: %v", err)
	}

	var target *introspect.Interface
	interfaceNames := make([]string, 0, len(node.Interfaces))
	for i := range node.Interfaces {
		interfaceNames = append(interfaceNames, node.Interfaces[i].Name)
		if node.Interfaces[i].Name == interfaceName {
			target = &node.Interfaces[i]
		}
	}
	if target == nil {
		t.Fatalf("introspection does not contain %s; got %v", interfaceName, interfaceNames)
	}

	actual := make(map[string]string, len(target.Methods))
	for _, method := range target.Methods {
		actual[method.Name] = methodSignature(method)
	}
	if len(actual) != len(want) {
		t.Fatalf("%s exposes %d methods, contract covers %d: %#v", interfaceName, len(actual), len(want), actual)
	}
	for method, expectedSignature := range want {
		actualSignature, exists := actual[method]
		if !exists {
			t.Errorf("%s does not expose %s", interfaceName, method)
			continue
		}
		if actualSignature != expectedSignature {
			t.Errorf("%s.%s signature = %q, want %q", interfaceName, method, actualSignature, expectedSignature)
		}
	}
}

func methodSignature(method introspect.Method) string {
	parts := make([]string, 0, len(method.Args))
	for _, arg := range method.Args {
		parts = append(parts, fmt.Sprintf("%s %s:%s", arg.Direction, arg.Name, arg.Type))
	}
	return strings.Join(parts, ", ")
}
