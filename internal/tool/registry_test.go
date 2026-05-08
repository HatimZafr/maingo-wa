package tool

import (
	"context"
	"os"
	"testing"
)

func TestRegistryScanYAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test_tool.yml"
	yaml := `
name: test_tool
description: A test tool
parameters:
  - name: input
    type: string
    required: true
    description: An input parameter
executor:
  type: shell
  command: "echo {{.input}}"
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(Config{
		DefinitionsDir:  dir,
		ShellTimeoutSec: 5,
		HTTPTimeoutSec:  5,
	})

	if err := reg.Scan(); err != nil {
		t.Fatal(err)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 tool, got %d", reg.Count())
	}

	tool, ok := reg.Get("test_tool")
	if !ok {
		t.Fatal("tool not found")
	}
	if tool.Metadata().Name != "test_tool" {
		t.Errorf("name = %q", tool.Metadata().Name)
	}
}

func TestRegistryExecuteShell(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/echo.yml"
	yaml := `
name: echo
description: Echo input
parameters:
  - name: text
    type: string
    required: true
    description: Text to echo
executor:
  type: shell
  command: "echo {{.text}}"
`
	os.WriteFile(path, []byte(yaml), 0644)

	reg := NewRegistry(Config{
		DefinitionsDir:  dir,
		ShellTimeoutSec: 5,
		HTTPTimeoutSec:  5,
	})
	if err := reg.Scan(); err != nil {
		t.Fatal(err)
	}

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
	dir := t.TempDir()
	path := dir + "/num.yml"
	yaml := `
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
`
	os.WriteFile(path, []byte(yaml), 0644)

	reg := NewRegistry(Config{
		DefinitionsDir:  dir,
		ShellTimeoutSec: 5,
		HTTPTimeoutSec:  5,
	})
	reg.Scan()

	_, err := reg.Execute(context.Background(), "num_tool", `{"count": "bukan_angka"}`)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidationRegex(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/regex_tool.yml"
	yaml := `
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
`
	os.WriteFile(path, []byte(yaml), 0644)

	reg := NewRegistry(Config{
		DefinitionsDir:  dir,
		ShellTimeoutSec: 5,
		HTTPTimeoutSec:  5,
	})
	reg.Scan()

	_, err := reg.Execute(context.Background(), "regex_tool", `{"url": "http://evil.com"}`)
	if err == nil {
		t.Fatal("expected validation error for non-https URL")
	}
}

func TestHTTPExecutorSSRFBlock(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/fetch.yml"
	yaml := `
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
`
	os.WriteFile(path, []byte(yaml), 0644)

	reg := NewRegistry(Config{
		DefinitionsDir:  dir,
		ShellTimeoutSec: 5,
		HTTPTimeoutSec:  5,
	})
	reg.Scan()

	_, err := reg.Execute(context.Background(), "fetch", `{"url": "http://127.0.0.1:8080/"}`)
	if err == nil {
		t.Fatal("expected SSRF block for localhost")
	}
}

func TestAllReturnsDefs(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/tool_a.yml"
	os.WriteFile(path, []byte(`
name: tool_a
description: A
parameters: []
executor:
  type: shell
  command: "true"
`), 0644)

	reg := NewRegistry(Config{DefinitionsDir: dir, ShellTimeoutSec: 5, HTTPTimeoutSec: 5})
	reg.Scan()

	defs := reg.All()
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Name != "tool_a" {
		t.Errorf("got %q", defs[0].Name)
	}
}
