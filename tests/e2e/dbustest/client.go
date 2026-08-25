//go:build e2e

// Package dbustest contains black-box D-Bus helpers shared by APM e2e tests.
package dbustest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

const defaultCallTimeout = 15 * time.Second

// ClientOption configures a D-Bus test client.
type ClientOption func(*Client)

// WithTimeout changes the timeout used by calls and job completion waits.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(client *Client) {
		client.timeout = timeout
	}
}

// CallJSON executes a method returning one JSON string and decodes it.
func CallJSON[T any](t testing.TB, request *Request) T {
	t.Helper()

	var raw string
	request.Store(t, &raw)
	return DecodeJSON[T](t, raw)
}

// DecodeJSON decodes a JSON document received over D-Bus.
func DecodeJSON[T any](t testing.TB, raw string) T {
	t.Helper()

	var result T
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode JSON response %q: %v", raw, err)
	}
	return result
}

// EncodeJSON encodes a complex D-Bus input as one JSON string.
func EncodeJSON(t testing.TB, value any) string {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON request: %v", err)
	}
	return string(raw)
}

// Client is a test client bound to one D-Bus destination and object path.
type Client struct {
	conn        *dbus.Conn
	destination string
	path        dbus.ObjectPath
	timeout     time.Duration
}

// NewSystemClient connects to the system bus. The connection is closed by t.Cleanup.
func NewSystemClient(t testing.TB, destination string, path dbus.ObjectPath, options ...ClientOption) *Client {
	t.Helper()

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		t.Fatalf("connect to system bus: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close system bus connection: %v", err)
		}
	})

	client := &Client{
		conn:        conn,
		destination: destination,
		path:        path,
		timeout:     defaultCallTimeout,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

// Request constructs a method call for the client's destination and object.
func (c *Client) Request(interfaceName, method string) *Request {
	return &Request{
		client: c,
		member: interfaceName + "." + method,
	}
}

// NameHasOwner reports whether the client's destination currently has an owner.
func (c *Client) NameHasOwner(t testing.TB) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var hasOwner bool
	if err := c.conn.BusObject().CallWithContext(
		ctx,
		"org.freedesktop.DBus.NameHasOwner",
		0,
		c.destination,
	).Store(&hasOwner); err != nil {
		t.Fatalf("query owner of %s: %v", c.destination, err)
	}
	return hasOwner
}

// Request is a reusable constructor for a single D-Bus method call.
type Request struct {
	client *Client
	member string
	flags  dbus.Flags
	args   []any
}

// Args sets the input arguments.
func (r *Request) Args(args ...any) *Request {
	r.args = args
	return r
}

// Flags sets D-Bus call flags, for example FlagAllowInteractiveAuthorization.
func (r *Request) Flags(flags dbus.Flags) *Request {
	r.flags = flags
	return r
}

// Call executes the request and returns the raw result for error assertions.
func (r *Request) Call() *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), r.client.timeout)
	defer cancel()

	return r.client.conn.Object(r.client.destination, r.client.path).CallWithContext(
		ctx,
		r.member,
		r.flags,
		r.args...,
	)
}

// Store executes the request and decodes its output arguments.
func (r *Request) Store(t testing.TB, destinations ...any) {
	t.Helper()

	call := r.Call()
	if call.Err != nil {
		t.Fatalf("call %s: %v", r.member, call.Err)
	}
	if err := call.Store(destinations...); err != nil {
		t.Fatalf("decode %s response: %v", r.member, err)
	}
}

// AssertError checks the exact remote D-Bus error name and body fragments.
func AssertError(t testing.TB, err error, wantName string, wantBody ...string) {
	t.Helper()

	if err == nil {
		t.Fatalf("D-Bus call unexpectedly succeeded; want error %s", wantName)
	}

	var dbusErr dbus.Error
	if !errors.As(err, &dbusErr) {
		t.Fatalf("got non-D-Bus error %T: %v; want %s", err, err, wantName)
	}
	if dbusErr.Name != wantName {
		t.Fatalf("D-Bus error = %q, want %q: %v", dbusErr.Name, wantName, err)
	}
	body := ""
	for _, value := range dbusErr.Body {
		body += fmt.Sprint(value)
	}
	for _, fragment := range wantBody {
		if !strings.Contains(body, fragment) {
			t.Fatalf("D-Bus error body %q does not contain %q", body, fragment)
		}
	}
}
