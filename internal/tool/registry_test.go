package tool

import (
	"context"
	"os"
	"testing"
)

func makeTestTool(t *testing.T, name, yaml string) (Tool, error) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		return nil, err
	}
	return ParseYAMLTool(path, Config{ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
}

func TestParseYAMLTool(t *testing.T) {
	yaml := `
name: test_tool
description: A test tool
parameters:
  - name: input
    type: string
    required: true
    description: Input
executor:
  type: shell
  command: "echo {{.input}}"
`
	tool, err := makeTestTool(t, "test_tool.yml", yaml)
	if err != nil {
		t.Fatal(err)
	}
	if tool.Metadata().Name != "test_tool" {
		t.Errorf("name = %q", tool.Metadata().Name)
	}
}

func TestRegistryExecuteShell(t *testing.T) {
	tool, _ := makeTestTool(t, "echo.yml", `
name: echo
description: Echo input
parameters:
  - name: text
    type: string
    required: true
    description: Text
executor:
  type: shell
  command: "echo {{.text}}"
`)
	reg := NewRegistry(Config{ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
	reg.Register(tool)

	result, err := reg.Execute(context.Background(), "echo", `{"text": "hello world"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world\n" {
		t.Errorf("got %q", result)
	}
}

func TestRegistryExecuteUnknownTool(t *testing.T) {
	reg := NewRegistry(Config{})
	_, err := reg.Execute(context.Background(), "nonexistent", "{}")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidationWrongType(t *testing.T) {
	tool, _ := makeTestTool(t, "num.yml", `
name: num_tool
description: Test
parameters:
  - name: count
    type: number
    required: true
    description: A number
executor:
  type: shell
  command: "echo {{.count}}"
`)
	reg := NewRegistry(Config{ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
	reg.Register(tool)

	_, err := reg.Execute(context.Background(), "num_tool", `{"count": "bukan_angka"}`)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidationRegex(t *testing.T) {
	tool, _ := makeTestTool(t, "regex.yml", `
name: regex_tool
description: Test
parameters:
  - name: url
    type: string
    required: true
    description: URL
    validation_regex: "^https://.*"
executor:
  type: shell
  command: "echo {{.url}}"
`)
	reg := NewRegistry(Config{ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
	reg.Register(tool)

	_, err := reg.Execute(context.Background(), "regex_tool", `{"url": "http://evil.com"}`)
	if err == nil {
		t.Fatal("expected validation error for non-https URL")
	}
}

func TestHTTPExecutorSSRFBlock(t *testing.T) {
	tool, _ := makeTestTool(t, "fetch.yml", `
name: fetch
description: Fetch URL
parameters:
  - name: url
    type: string
    required: true
    description: URL
executor:
  type: http
  method: GET
  url: "{{.url}}"
`)
	reg := NewRegistry(Config{ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
	reg.Register(tool)

	_, err := reg.Execute(context.Background(), "fetch", `{"url": "http://127.0.0.1:8080/"}`)
	if err == nil {
		t.Fatal("expected SSRF block for localhost")
	}
}

func TestAllReturnsDefs(t *testing.T) {
	tool, _ := makeTestTool(t, "tool_a.yml", `
name: tool_a
description: A
parameters: []
executor:
  type: shell
  command: "true"
`)
	reg := NewRegistry(Config{ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
	reg.Register(tool)

	defs := reg.All()
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Name != "tool_a" {
		t.Errorf("got %q", defs[0].Name)
	}
}
